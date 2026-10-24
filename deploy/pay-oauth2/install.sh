#!/usr/bin/env bash
# pay-oauth2 一键部署 / 更新（幂等，可重复执行）
# 用法：bash install.sh            # 部署 + 自检
#       bash install.sh --clean    # 卸载（移除 nginx 片段与 fpm pool，保留数据表）
set -euo pipefail

APP_DIR=/www/wwwroot/pay-oauth2
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PHP=/www/server/php/81/bin/php
FPM_CONF=/www/server/php/81/etc/php-fpm.conf
FPM_POOL_DIR=/www/server/php/81/etc/php-fpm.d
NGINX_EXT=/www/server/panel/vhost/nginx/extension/pay
NGINX=/www/server/nginx/sbin/nginx
PAY_DB_USER=$(awk '/^database:/{f=1;next} /^[a-z]/{f=0} f&&/^\s*user:/{print $2}' /www/wwwroot/pay/config/config.yaml | tr -d '"' | head -1)
PAY_DB_PASS=$(awk '/^database:/{f=1;next} /^[a-z]/{f=0} f&&/^\s*password:/{print $2}' /www/wwwroot/pay/config/config.yaml | tr -d '"' | head -1)
PAY_DB_NAME=$(awk '/^database:/{f=1;next} /^[a-z]/{f=0} f&&/^\s*dbname:/{print $2}' /www/wwwroot/pay/config/config.yaml | tr -d '"' | head -1)

if [[ "${1:-}" == "--clean" ]]; then
    echo "== 卸载 nginx 片段与 fpm pool（数据表保留） =="
    rm -f "$NGINX_EXT/oauth2.conf"
    "$NGINX" -t && "$NGINX" -s reload
    rm -f "$FPM_POOL_DIR/pay-oauth2.conf"
    sed -i '\#include = /www/server/php/81/etc/php-fpm.d/\*.conf#d' "$FPM_CONF"
    systemctl reload php-fpm-81 || true
    echo "已卸载。删除 $APP_DIR 与本插件数据表可彻底清理。"
    exit 0
fi

echo "== 1/6 目录与权限 =="
mkdir -p "$APP_DIR"/{lib,assets,bin,tools,var}
if [[ "$SRC_DIR" != "$APP_DIR" ]]; then
    cp -r "$SRC_DIR"/index.php "$SRC_DIR"/config.php "$SRC_DIR"/install.sql "$APP_DIR"/
    cp -r "$SRC_DIR"/lib/. "$APP_DIR"/lib/
    cp -r "$SRC_DIR"/assets/. "$APP_DIR"/assets/
    cp -r "$SRC_DIR"/bin/. "$APP_DIR"/bin/
    cp -r "$SRC_DIR"/tools/. "$APP_DIR"/tools/
fi
chown -R www:www "$APP_DIR"
chmod 700 "$APP_DIR/var"
chmod 640 "$APP_DIR/config.php" || true

echo "== 2/6 语法检查 =="
for f in "$APP_DIR"/index.php "$APP_DIR"/lib/*.php "$APP_DIR"/bin/doctor.php; do
    "$PHP" -l "$f" >/dev/null
done
echo "PHP 语法 OK"

echo "== 3/6 建表 =="
if [[ -n "$PAY_DB_NAME" && -n "$PAY_DB_USER" ]]; then
    mysql -h127.0.0.1 -u"$PAY_DB_USER" -p"$PAY_DB_PASS" "$PAY_DB_NAME" < "$APP_DIR/install.sql"
    echo "数据表就绪"
else
    echo "跳过（未能从 pay config.yaml 解析出数据库信息，请手工导入 install.sql）"
fi

echo "== 4/6 php-fpm 专用 pool（127.0.0.1:9101） =="
mkdir -p "$FPM_POOL_DIR"
cp "$SRC_DIR/php-fpm-pool.conf" "$FPM_POOL_DIR/pay-oauth2.conf"
grep -q 'php-fpm.d/\*.conf' "$FPM_CONF" || echo 'include = /www/server/php/81/etc/php-fpm.d/*.conf' >> "$FPM_CONF"
/www/server/php/81/sbin/php-fpm -t -y "$FPM_CONF"
systemctl reload php-fpm-81 || systemctl restart php-fpm-81
sleep 1

echo "== 5/6 nginx 片段 =="
mkdir -p "$NGINX_EXT"
cp "$SRC_DIR/nginx.oauth2.conf" "$NGINX_EXT/oauth2.conf"
chmod 600 "$NGINX_EXT/oauth2.conf"
"$NGINX" -t
"$NGINX" -s reload

echo "== 6/6 自检 =="
curl -s -H 'Host: pay.mcapple.top' http://127.0.0.1/oauth2/health || true
echo
"$PHP" "$APP_DIR/bin/doctor.php" || true
echo
echo "部署完成。请编辑 $APP_DIR/config.php 填写 IdP 参数后：systemctl reload php-fpm-81"
