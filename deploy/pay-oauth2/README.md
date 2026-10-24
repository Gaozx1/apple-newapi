# pay-oauth2

给 `ls-go`（聚合免签系统，`/www/wwwroot/pay`，监听 `127.0.0.1:19855`）的**用户中心**加上标准 **OAuth 2.0 / OpenID Connect** 登录。
主程序是编译好的 Go 二进制（无源码、无登录插件钩子），因此采用「旁路接入 + 复用原生会话」的方式：**不改二进制、不改既有表结构、不影响原有账号密码登录**。

## 一、原理（实测结论，不是猜的）

| 项目 | 结论 |
| --- | --- |
| 会话凭证 | Cookie `front_token`，值等于 `ls_user.token` |
| 兼容方式 | 同时接受 `Authorization: Bearer <token>`（`Authorization: <token>` 亦可） |
| 过期控制 | `ls_user.token_expire`（Unix 秒），过期返回 `{"code":401,"msg":"登录已过期"}` |
| 登录有效期 | `admin_config.user_login_token_hours`（本机当前 168 小时） |
| 用户中心 API | `/FrontApi/currentUser`、`/FrontApi/accountProfile` 等（Cookie 鉴权） |
| SPA 行为 | axios 对 `/FrontApi|/User|/Pay|/Notify` 前缀**不发** Authorization，只带 Cookie |

所以 SSO 落地方式：OAuth2 回调成功后，把 IdP 身份映射到本地 `ls_user`，写入随机 `token` + `token_expire`，再下发 `front_token` Cookie —— 对 ls-go 而言与「刚用密码登录成功」完全等价。

## 二、目录

```
/www/wwwroot/pay-oauth2/
├── index.php             # 前端控制器：/login /callback /logout /health /assets/*
├── config.php            # 唯一需要改的文件（IdP、账号策略、Cookie）
├── install.sh            # 幂等部署脚本（bash install.sh / --clean）
├── install.sql           # 三张附加表
├── nginx.oauth2.conf     # 站点 extension 片段
├── php-fpm-pool.conf     # 专用 fpm pool（127.0.0.1:9101）
├── lib/
│   ├── Support.php       # 编码/随机/Cookie/安全头/站内跳转校验
│   ├── Config.php        # 配置 + pay 的 config.yaml 解析
│   ├── Database.php      # PDO（复用 pay 数据库凭据）
│   ├── Audit.php         # 审计 + 固定窗口限流
│   ├── OAuth2Client.php  # discovery / PKCE / token / JWKS / id_token 验签
│   └── PayAccount.php    # 身份映射、建号、签发会话
├── assets/sso.js         # 登录页入口注入（不改 SPA 构建产物）
├── assets/config.js      # 动态生成（未启用时前端不显示按钮）
├── bin/doctor.php        # 自检
├── tools/mock-idp.php    # 本地最小 OIDC，仅用于自测
└── var/                  # secret.key、oidc/jwks 缓存、日志（0700/0600）
```

## 三、部署

```bash
cd /www/wwwroot/pay-oauth2
vi config.php                      # 填 enabled / oauth.issuer / oauth.client_id / oauth.client_secret
bash install.sh                    # 目录、权限、建表、fpm pool、nginx、自检
bash install.sh --clean            # 卸载（移除 nginx 片段与 fpm pool，保留数据表）
```

`install.sh` 会自动：从 `/www/wwwroot/pay/config/config.yaml` 读数据库凭据建表 → 建 php-fpm pool `[pay-oauth2]`（`127.0.0.1:9101`）→ 装 nginx 片段 → `nginx -t && reload` → 打 `/oauth2/health` + `doctor.php`。

### 为什么用 php-fpm + TCP 而不是默认 socket

本机 nginx 与默认 pool 的 `/tmp/php-cgi-81.sock` 处于**不同挂载命名空间**（nginx 侧直接 `connect() ... No such file or directory`，502）。因此新建独立 pool 监听 `127.0.0.1:9101`，nginx 走同 netns 的 TCP，稳定且与站点 `[www]` pool 互不影响。

### IdP 侧要登记的回调地址

```
https://pay.mcapple.top/oauth2/callback
```

`config.php` 里 `oauth.redirect_uri` 留空时按当前域名自动推导，显式填写更稳。

### 常见 IdP 填法

| IdP | issuer | 备注 |
| --- | --- | --- |
| Logto | `https://<tenant>.logto.app/oidc` | scope 勾 `openid profile email` |
| Authentik | `https://sso.example.com/application/o/<slug>/` | Provider 选 OIDC |
| Keycloak | `https://kc.example.com/realms/<realm>` | Client 打开 PKCE(S256) |
| Casdoor | `https://door.example.com` | 开 discovery |
| Google | `https://accounts.google.com` | Web 应用类型 |
| 纯 OAuth2（GitHub 等） | 关 `discovery`，手填端点 | `verify_id_token=false`，靠 userinfo 取身份 |

字段名不一致时改 `oauth.claims`（`subject/username/email/name`）。

## 四、登录流程

```
用户中心 /user/#/login
  └─(点「使用 SSO 一键登录」)→ /oauth2/login?redirect=/user/
       ├─ 生成 state / nonce / PKCE verifier，HMAC 封签进 ls_sso_tx Cookie（10 分钟）
       └─ 302 → IdP authorize
  IdP 认证通过 → 302 /oauth2/callback?code&state
       ├─ hash_equals 校验 state，回调后立即清 tx Cookie
       ├─ code + verifier 换 token（client_secret_post / basic）
       ├─ 校验 id_token：alg 白名单 + JWKS 验签 + iss/aud/exp/nbf/nonce
       ├─ userinfo 补全 preferred_username / email
       ├─ 映射身份：ls_oauth_identity → ls_user；缺失则建号（bcrypt 随机密码）
       ├─ 写 ls_user.token / token_expire
       └─ Set-Cookie: front_token=...; HttpOnly; SameSite=Lax; Secure
  → 302 回 /user/，SPA 凭 Cookie 直接进入登录态
```

附加接口：`/oauth2/login?mode=bind`（已登录的本地账号绑定 SSO 身份）、`/oauth2/logout`、`/oauth2/health`。

## 五、安全实现要点

- 授权码 + **PKCE(S256)**；机密客户端另用 `client_secret`；`state` 用 `hash_equals` 比对；`nonce` 参与 id_token 校验。
- 事务 Cookie `ls_sso_tx` 用 `var/secret.key` 做 HMAC-SHA256 封签，`HttpOnly` + `SameSite=Lax`，10 分钟过期，回调即清。
- id_token 仅接受 `RS256/ES256/HS256`，公钥取自 IdP JWKS（缓存 30 分钟），校验 `iss/aud/exp/nbf/nonce`；失败即拒绝，不降级。
- 开放重定向防护：仅站内相对路径（拒绝 `//`、`/\`、控制字符）+ `security.redirect_allow_prefixes` 白名单。
- `account.link_by_email` 默认关闭；开启时只接受 `email_verified=1`，否则存在邮箱接管风险。
- 单 IP 限流（`ls_oauth_rate`，默认 20 次/分钟）；审计写 `ls_oauth_audit`，**不记录** code / token / secret / 密码。
- 退出登录同时清 Cookie 并清空 `ls_user.token`，不只清浏览器。
- 新建账号默认密码为 16 字节随机值的 bcrypt 哈希，无人知晓，只能走 SSO（可用 `freeze_new_user` 强制人工审核）。

## 六、运维

```bash
# 健康检查（不返回密钥）
curl -s https://pay.mcapple.top/oauth2/health

# 自检
/www/server/php/81/bin/php /www/wwwroot/pay-oauth2/bin/doctor.php

# 日志
tail -f /www/wwwroot/pay-oauth2/var/oauth2.log
tail -f /www/wwwroot/pay-oauth2/var/php-error.log

# 审计 / 解绑
mysql -upay -p pay -e "select * from ls_oauth_audit order by id desc limit 20;"
mysql -upay -p pay -e "delete from ls_oauth_identity where user_id=<ID>;"
```

**回滚**：`bash install.sh --clean`；主程序与会话机制不受影响。彻底清理再 `drop table ls_oauth_identity, ls_oauth_audit, ls_oauth_rate;`

## 七、已验证项（本机实测）

用 `tools/mock-idp.php`（本地 HS256 OIDC）跑通完整链路：

| 用例 | 结果 |
| --- | --- |
| `/oauth2/login` | 302 → IdP authorize，带 state/nonce/PKCE + 封签 Cookie |
| IdP 回调 → `/oauth2/callback` | 302 → `/user/`，下发 `front_token`（HttpOnly/SameSite=Lax/168h） |
| `/FrontApi/currentUser`（Cookie 会话） | 200，返回新建账号 |
| 自动建号 | `ls_user` + `ls_userbasic` + `ls_oauth_identity` 三处一致，`token` 36 位、未过期 |
| 第二个身份登录 | 新建独立账号（用户名/邀请码唯一性正常） |
| state 伪造 | 400 拒绝 |
| 授权码重放 | 非 200 拒绝（IdP 判 invalid_grant） |
| 退出登录 | Cookie 清除 + `ls_user.token` 置空，会话立即失效 |
| 原密码登录 | 不受影响（仍返回验证码流程） |
