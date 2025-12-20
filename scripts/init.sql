-- 数据中间件数据库初始化脚本

-- 创建数据库 (如果不存在)
CREATE DATABASE IF NOT EXISTS datamiddleware CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE datamiddleware;

-- 玩家表
CREATE TABLE IF NOT EXISTS players (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL UNIQUE COMMENT '用户ID',
    game_id VARCHAR(64) NOT NULL COMMENT '游戏ID',
    username VARCHAR(64) NOT NULL COMMENT '用户名',
    password VARCHAR(256) NOT NULL COMMENT '密码哈希',
    email VARCHAR(128) COMMENT '邮箱',
    phone VARCHAR(32) COMMENT '手机号',
    nickname VARCHAR(64) COMMENT '昵称',
    avatar VARCHAR(256) COMMENT '头像URL',
    level INT DEFAULT 1 COMMENT '等级',
    experience BIGINT DEFAULT 0 COMMENT '经验值',
    coins BIGINT DEFAULT 0 COMMENT '金币',
    diamonds BIGINT DEFAULT 0 COMMENT '钻石',
    status VARCHAR(16) DEFAULT 'active' COMMENT '状态: active, banned, deleted',
    last_login_at TIMESTAMP NULL COMMENT '最后登录时间',
    last_login_ip VARCHAR(64) COMMENT '最后登录IP',
    device_id VARCHAR(128) COMMENT '设备ID',
    platform VARCHAR(16) COMMENT '平台: ios, android, web',
    version VARCHAR(32) COMMENT '客户端版本',
    extra_data TEXT COMMENT '额外数据(JSON)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_game_user (game_id, user_id),
    INDEX idx_username (username),
    INDEX idx_status (status),
    INDEX idx_last_login (last_login_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 道具表
CREATE TABLE IF NOT EXISTS items (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    item_id VARCHAR(64) NOT NULL COMMENT '道具ID',
    game_id VARCHAR(64) NOT NULL COMMENT '游戏ID',
    name VARCHAR(128) NOT NULL COMMENT '道具名称',
    type VARCHAR(32) NOT NULL COMMENT '道具类型',
    description TEXT COMMENT '道具描述',
    price BIGINT DEFAULT 0 COMMENT '价格',
    quantity INT DEFAULT 1 COMMENT '数量',
    max_quantity INT DEFAULT 999 COMMENT '最大数量',
    status VARCHAR(16) DEFAULT 'active' COMMENT '状态: active, inactive',
    extra_data TEXT COMMENT '额外数据(JSON)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    UNIQUE KEY uk_game_item (game_id, item_id),
    INDEX idx_game_type (game_id, type),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 订单表
CREATE TABLE IF NOT EXISTS orders (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    order_id VARCHAR(64) NOT NULL UNIQUE COMMENT '订单ID',
    game_id VARCHAR(64) NOT NULL COMMENT '游戏ID',
    user_id VARCHAR(64) NOT NULL COMMENT '用户ID',
    item_id VARCHAR(64) NOT NULL COMMENT '道具ID',
    quantity INT NOT NULL COMMENT '数量',
    total_price BIGINT NOT NULL COMMENT '总价',
    currency VARCHAR(16) DEFAULT 'CNY' COMMENT '货币类型',
    status VARCHAR(16) DEFAULT 'pending' COMMENT '状态: pending, paid, delivered, cancelled',
    payment_method VARCHAR(32) COMMENT '支付方式',
    payment_id VARCHAR(128) COMMENT '支付ID',
    delivery_time TIMESTAMP NULL COMMENT '交付时间',
    extra_data TEXT COMMENT '额外数据(JSON)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_game_user (game_id, user_id),
    INDEX idx_status (status),
    INDEX idx_created (created_at),
    INDEX idx_payment (payment_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 游戏表
CREATE TABLE IF NOT EXISTS games (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    game_id VARCHAR(64) NOT NULL UNIQUE COMMENT '游戏ID',
    name VARCHAR(128) NOT NULL COMMENT '游戏名称',
    description TEXT COMMENT '游戏描述',
    status VARCHAR(16) DEFAULT 'active' COMMENT '状态: active, inactive',
    max_players INT DEFAULT 1000 COMMENT '最大玩家数',
    current_players INT DEFAULT 0 COMMENT '当前玩家数',
    version VARCHAR(32) COMMENT '版本号',
    extra_data TEXT COMMENT '额外数据(JSON)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_status (status),
    INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 插入一些测试数据
INSERT IGNORE INTO games (game_id, name, description, max_players) VALUES
('game1', '游戏1', '测试游戏1', 1000),
('game2', '游戏2', '测试游戏2', 800);

INSERT IGNORE INTO items (item_id, game_id, name, type, description, price, quantity) VALUES
('item001', 'game1', '金币包', 'currency', '1000金币', 100, 1000),
('item002', 'game1', '钻石包', 'currency', '100钻石', 1000, 100),
('item003', 'game2', '经验药水', 'consumable', '增加经验值', 50, 500);

-- 创建用户 (如果需要)
-- 注意: 密码应该是bcrypt哈希后的值
-- 这里只是示例，实际部署时应该使用安全的方式创建用户
