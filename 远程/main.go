// 远程控制服务 —— 被控端运行本程序，控制端浏览器输入控制码即可连接操作。
// 仅使用 Go 标准库 + Windows API，无第三方依赖。
package main

import (
	_ "embed"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed index.html
var indexHTML []byte

const (
	defaultPort    = 8888
	sessionMaxAge  = 12 * time.Hour
	cookieName     = "rc_token"
	maxFailsPerIP  = 5
	blockDuration  = 15 * time.Minute
	configFileName = "remote-config.json"
)

type config struct {
	Code string `json:"code"`
	Port int    `json:"port"`
}

// sessionInfo 记录一个控制端会话，用于设备列表与踢出功能
type sessionInfo struct {
	IP          string
	UA          string
	ConnectedAt time.Time
	LastSeen    time.Time
}

var (
	cfg     config
	cfgPath string

	cfgMu    sync.Mutex
	sessMu   sync.Mutex
	sessions = map[string]*sessionInfo{} // token -> 会话信息
	kicked   = map[string]time.Time{}    // 被踢出的 token -> 踢出时间（用于返回明确提示）

	failMu    sync.Mutex
	loginFail = map[string]*failState{} // ip -> 失败记录
)

type failState struct {
	fails        int
	blockedUntil time.Time
}

func main() {
	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		exeDir, _ = os.Getwd()
	}
	flagPort := flag.Int("port", 0, "监听端口，默认 8888")
	flagCode := flag.String("code", "", "手动指定控制码（纯数字，留空则自动生成）")
	flag.Parse()

	cfgPath = filepath.Join(exeDir, configFileName)
	loadConfig(*flagCode, *flagPort)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/info", handleInfo)
	mux.HandleFunc("/api/connect", handleConnect)
	mux.HandleFunc("/api/disconnect", handleDisconnect)
	mux.HandleFunc("/api/ping", auth(handlePing))
	mux.HandleFunc("/api/stream", auth(handleStream))
	mux.HandleFunc("/api/frame", auth(handleFrame))
	mux.HandleFunc("/api/cmd", auth(handleCmd))
	mux.HandleFunc("/api/regen", localOnly(handleRegen))
	mux.HandleFunc("/api/kick", localOnly(handleKick))

	printBanner()
	addr := fmt.Sprintf(":%d", cfg.Port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("监听 %s 失败: %v", addr, err)
	}
}

// ---------- 配置 ----------

func loadConfig(codeFlag string, portFlag int) {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	cfg = config{Code: genCode(), Port: defaultPort}
	if data, err := os.ReadFile(cfgPath); err == nil {
		var saved config
		if json.Unmarshal(data, &saved) == nil && isValidCode(saved.Code) && saved.Port > 0 {
			cfg = saved
		}
	}
	if isValidCode(codeFlag) {
		cfg.Code = codeFlag
	}
	if portFlag > 0 {
		cfg.Port = portFlag
	}
	saveConfigLocked()
}

func saveConfigLocked() {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(cfgPath, data, 0600); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
}

func genCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "888888"
	}
	return strconv.Itoa(int(n.Int64()) + 100000)
}

func isValidCode(s string) bool {
	if len(s) < 4 || len(s) > 10 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ---------- 通用 ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func failJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": msg})
}

func isLocalRequest(r *http.Request) bool {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func localIPs() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
				out = append(out, ipnet.IP.String())
			}
		}
	}
	return out
}

func virtualScreenSize() (int, int) {
	_, _, vw, vh := virtualScreenRect()
	return vw, vh
}

// ---------- 鉴权 ----------

func checkSession(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return false
	}
	sessMu.Lock()
	defer sessMu.Unlock()
	info, ok := sessions[c.Value]
	if !ok {
		return false
	}
	if time.Now().After(info.LastSeen.Add(sessionMaxAge)) {
		delete(sessions, c.Value)
		return false
	}
	info.LastSeen = time.Now() // 滑动续期
	return true
}

func auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkSession(r) {
			msg := "未连接或连接已过期，请重新输入控制码"
			if c, err := r.Cookie(cookieName); err == nil {
				sessMu.Lock()
				if _, wasKicked := kicked[c.Value]; wasKicked {
					msg = "该设备已被管理员断开连接"
				}
				sessMu.Unlock()
			}
			failJSON(w, http.StatusUnauthorized, msg)
			return
		}
		next(w, r)
	}
}

func localOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLocalRequest(r) {
			failJSON(w, http.StatusForbidden, "仅本机可操作")
			return
		}
		next(w, r)
	}
}

// deviceID 取 token 前 8 位十六进制作为设备标识
func deviceID(token string) string { return token[:8] }

func deviceList() []map[string]any {
	now := time.Now()
	out := []map[string]any{}
	for token, info := range sessions {
		if now.After(info.LastSeen.Add(sessionMaxAge)) {
			continue
		}
		out = append(out, map[string]any{
			"id":    deviceID(token),
			"ip":    info.IP,
			"ua":    info.UA,
			"since": info.ConnectedAt.Unix(),
		})
	}
	return out
}

// ---------- 处理器 ----------

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(indexHTML)
}

func handleInfo(w http.ResponseWriter, r *http.Request) {
	local := isLocalRequest(r)
	authed := checkSession(r)

	resp := map[string]any{
		"ok":     true,
		"local":  local,
		"authed": authed,
		"port":   cfg.Port,
	}
	if authed || local {
		vx, vy, vw, vh := virtualScreenRect()
		resp["vx"], resp["vy"] = vx, vy
		resp["screenW"], resp["screenH"] = vw, vh
		sessMu.Lock()
		resp["clients"] = len(sessions)
		sessMu.Unlock()
	}
	if local {
		resp["code"] = cfg.Code
		resp["ips"] = localIPs()
		sessMu.Lock()
		resp["devices"] = deviceList()
		sessMu.Unlock()
	}
	writeJSON(w, 200, resp)
}

func handleConnect(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	failMu.Lock()
	fs := loginFail[ip]
	if fs != nil && time.Now().Before(fs.blockedUntil) {
		remain := int(time.Until(fs.blockedUntil).Seconds())
		failMu.Unlock()
		failJSON(w, http.StatusTooManyRequests, fmt.Sprintf("尝试次数过多，请 %d 秒后再试", remain))
		return
	}
	failMu.Unlock()

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		failJSON(w, 400, "请求格式错误")
		return
	}
	req.Code = strings.TrimSpace(req.Code)

	// 哈希后常数时间比较，避免时序侧信道
	got := sha256.Sum256([]byte(req.Code))
	want := sha256.Sum256([]byte(cfg.Code))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		failMu.Lock()
		if loginFail[ip] == nil {
			loginFail[ip] = &failState{}
		}
		fs := loginFail[ip]
		fs.fails++
		if fs.fails >= maxFailsPerIP {
			fs.blockedUntil = time.Now().Add(blockDuration)
			fs.fails = 0
		}
		failMu.Unlock()
		time.Sleep(400 * time.Millisecond) // 拖慢爆破
		log.Printf("控制码错误 (来自 %s)", ip)
		failJSON(w, 401, "控制码错误")
		return
	}

	failMu.Lock()
	delete(loginFail, ip)
	failMu.Unlock()

	token := make([]byte, 16)
	rand.Read(token)
	tokenStr := hex.EncodeToString(token)
	sessMu.Lock()
	sessions[tokenStr] = &sessionInfo{
		IP:          ip,
		UA:          r.UserAgent(),
		ConnectedAt: time.Now(),
		LastSeen:    time.Now(),
	}
	sessMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: tokenStr, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 86400,
	})
	log.Printf("控制端已连接 (来自 %s)", ip)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		sessMu.Lock()
		delete(sessions, c.Value)
		sessMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	vw, vh := virtualScreenSize()
	sessMu.Lock()
	clients := len(sessions)
	sessMu.Unlock()
	writeJSON(w, 200, map[string]any{"ok": true, "clients": clients, "screenW": vw, "screenH": vh})
}

func handleRegen(w http.ResponseWriter, r *http.Request) {
	cfgMu.Lock()
	cfg.Code = genCode()
	saveConfigLocked()
	code := cfg.Code
	cfgMu.Unlock()
	log.Printf("控制码已重新生成")
	writeJSON(w, 200, map[string]any{"ok": true, "code": code})
}

// handleKick 管理员（本机页面）断开某个已连接的控制端设备
func handleKick(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		failJSON(w, 400, "缺少设备 ID")
		return
	}
	sessMu.Lock()
	removed := 0
	for token := range sessions {
		if deviceID(token) == req.ID {
			delete(sessions, token)
			kicked[token] = time.Now()
			removed++
		}
	}
	for token, at := range kicked { // 清理过期记录
		if time.Since(at) > time.Hour {
			delete(kicked, token)
		}
	}
	sessMu.Unlock()
	if removed == 0 {
		failJSON(w, 404, "设备不存在或已断开")
		return
	}
	log.Printf("设备 %s 已被踢出 (来自 %s)", req.ID, clientIP(r))
	writeJSON(w, 200, map[string]any{"ok": true, "removed": removed})
}

func handleStream(w http.ResponseWriter, r *http.Request) {
	targetW := parseBounded(r, "w", 0, 0, 3840)
	quality := parseBounded(r, "q", 65, 30, 92)

	flusher, ok := w.(http.Flusher)
	if !ok {
		failJSON(w, 500, "当前服务器不支持流式响应")
		return
	}
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=\"frame\"")
	w.Header().Set("Cache-Control", "no-store")

	interval := time.Second / 12
	for {
		if !checkSession(r) { // 每帧校验，踢出后立即停止画面
			return
		}
		jpegBytes, _, _, err := CaptureJPEG(targetW, quality)
		if err != nil {
			log.Printf("截屏失败: %v", err)
			return
		}
		fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(jpegBytes))
		if _, err := w.Write(jpegBytes); err != nil {
			return
		}
		fmt.Fprint(w, "\r\n")
		flusher.Flush()

		select {
		case <-r.Context().Done():
			return
		case <-time.After(interval):
		}
	}
}

func handleFrame(w http.ResponseWriter, r *http.Request) {
	targetW := parseBounded(r, "w", 0, 0, 3840)
	quality := parseBounded(r, "q", 65, 30, 92)
	jpegBytes, _, _, err := CaptureJPEG(targetW, quality)
	if err != nil {
		failJSON(w, 500, "截屏失败: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(jpegBytes)
}

type command struct {
	Type   string   `json:"type"` // move/click/down/up/wheel/key/text/paste
	X      int      `json:"x"`
	Y      int      `json:"y"`
	Button string   `json:"button"`
	Amount int      `json:"amount"`
	Horiz  bool     `json:"horiz"`
	Key    string   `json:"key"`
	Mods   []string `json:"mods"`
	Text   string   `json:"text"`
}

func handleCmd(w http.ResponseWriter, r *http.Request) {
	var cmd command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		failJSON(w, 400, "请求格式错误")
		return
	}

	var err error
	switch cmd.Type {
	case "move":
		err = MouseMove(cmd.X, cmd.Y)
	case "down", "up":
		err = MouseButton(cmd.Button, cmd.Type == "down")
	case "wheel":
		err = MouseWheel(cmd.Amount, cmd.Horiz)
	case "key":
		err = SendKeyCombo(cmd.Key, cmd.Mods)
	case "text":
		err = SendText(cmd.Text)
	case "paste":
		err = PasteText(cmd.Text)
	default:
		failJSON(w, 400, "未知指令: "+cmd.Type)
		return
	}
	if err != nil {
		log.Printf("指令 %s 执行失败: %v", cmd.Type, err)
		failJSON(w, 500, "指令执行失败: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func parseBounded(r *http.Request, key string, def, min, max int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

// ---------- 启动信息 ----------

func printBanner() {
	ips := localIPs()
	log.SetFlags(log.LstdFlags)
	fmt.Println("==================================================")
	fmt.Println("  远程控制服务已启动")
	fmt.Printf("  控制码:  %s\n", cfg.Code)
	fmt.Printf("  端口:    %d\n", cfg.Port)
	fmt.Printf("  本机访问:  http://localhost:%d\n", cfg.Port)
	for _, ip := range ips {
		fmt.Printf("  控制端访问: http://%s:%d\n", ip, cfg.Port)
	}
	fmt.Printf("  配置文件:  %s\n", cfgPath)
	fmt.Println("  提示: 控制端与被控端需网络互通，首次运行请允许防火墙。")
	fmt.Println("==================================================")
}
