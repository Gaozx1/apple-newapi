-- pay-oauth2 附加表结构
-- 只新增表，不改动 ls-go 的任何既有表结构；MySQL 5.7 / 8.0 通用。

CREATE TABLE IF NOT EXISTS `ls_oauth_identity` (
  `id`             int(11) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `provider`       varchar(64)  NOT NULL DEFAULT '' COMMENT 'IdP 标识（issuer+client_id 摘要）',
  `subject`        varchar(191) NOT NULL DEFAULT '' COMMENT 'IdP 用户唯一标识（sub）',
  `user_id`        int(11) unsigned NOT NULL DEFAULT '0' COMMENT '本地 ls_user.id',
  `username`       varchar(100) NOT NULL DEFAULT '' COMMENT 'IdP 用户名快照',
  `email`          varchar(191) NOT NULL DEFAULT '' COMMENT 'IdP 邮箱快照',
  `email_verified` tinyint(1)   NOT NULL DEFAULT '0' COMMENT '邮箱是否已验证',
  `created_at`     datetime     DEFAULT NULL COMMENT '绑定时间',
  `last_login_at`  datetime     DEFAULT NULL COMMENT '最近 SSO 登录',
  `last_ip`        varchar(64)  NOT NULL DEFAULT '' COMMENT '最近登录 IP',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_provider_subject` (`provider`, `subject`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='OAuth2 身份绑定';

CREATE TABLE IF NOT EXISTS `ls_oauth_audit` (
  `id`           bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
  `event`        varchar(32)  NOT NULL DEFAULT '' COMMENT '事件',
  `result`       varchar(32)  NOT NULL DEFAULT '' COMMENT '结果',
  `subject_hash` varchar(32)  NOT NULL DEFAULT '' COMMENT 'sub 的 SHA-256 摘要前 32 位',
  `user_id`      int(11) unsigned NOT NULL DEFAULT '0' COMMENT '本地用户 ID',
  `ip`           varchar(64)  NOT NULL DEFAULT '' COMMENT '来源 IP',
  `detail`       varchar(500) NOT NULL DEFAULT '' COMMENT '非敏感细节',
  `created_at`   datetime     DEFAULT NULL COMMENT '时间',
  PRIMARY KEY (`id`),
  KEY `idx_created` (`created_at`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='OAuth2 登录审计';

CREATE TABLE IF NOT EXISTS `ls_oauth_rate` (
  `bucket`       varchar(64) NOT NULL COMMENT '限流桶（IP 摘要 + 用途）',
  `window_start` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '窗口起点',
  `hits`         int(11) unsigned NOT NULL DEFAULT '0' COMMENT '命中次数',
  PRIMARY KEY (`bucket`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='OAuth2 限流计数';
