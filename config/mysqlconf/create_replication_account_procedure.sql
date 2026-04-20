USE mysql;

delimiter //

-- username      主从复制账号名称
-- userpassword  主从复制账号密码
CREATE PROCEDURE IF NOT EXISTS CreateDumpUser(IN username VARCHAR(512), IN userpassword VARCHAR(512))
BEGIN
    DECLARE user_exists INT;

    SELECT COUNT(*) INTO user_exists FROM mysql.user WHERE user = username;
    
    IF user_exists = 0 THEN
        -- 创建用户
        SET @create_user_sql = CONCAT('CREATE USER ''', username, '''@''%'' IDENTIFIED WITH caching_sha2_password by ''', userpassword, '''');
        PREPARE create_user_stmt FROM @create_user_sql;
        EXECUTE create_user_stmt;
        DEALLOCATE PREPARE create_user_stmt;
        SELECT CONCAT('create user:', username, ' success') AS message;

        -- 授予复制权限
        SET @grant_sql = CONCAT('GRANT replication slave ON *.* TO ''', username, '''@''%''');
        PREPARE grant_stmt FROM @grant_sql;
        EXECUTE grant_stmt;
        DEALLOCATE PREPARE grant_stmt;
        SELECT CONCAT('grant replication slave to user:', username, ' success');
        
    
        flush privileges;
    ELSE
        SELECT CONCAT('user:', username, ' already exists') AS message;
    END IF;
END // 

DELIMITER ;