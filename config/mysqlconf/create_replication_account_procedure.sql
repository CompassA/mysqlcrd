USE mysql;

delimiter //

-- username       主从复制账号名称
-- userpassword   主从复制账号密码
-- enablesemisync 0: 不启动半同步
CREATE PROCEDURE IF NOT EXISTS CreateDumpUser(IN username VARCHAR(512), IN userpassword VARCHAR(512), IN enablesemisync INT(11))
BEGIN
    DECLARE user_exists INT;
    DECLARE semi_sync_source_active INT;

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

    IF enablesemisync > 0 THEN 
        -- 启动半同步, 配置半同步需等待的从节点数量
        -- https://dev.mysql.com/doc/refman/8.4/en/replication-semisync-installation.html
        SET GLOBAL rpl_semi_sync_source_enabled = 1;
        SET @set_global_semi_cnt = CONCAT('SET GLOBAL rpl_semi_sync_source_wait_for_replica_count = ', enablesemisync);
        PREPARE set_global_semi_cnt_stmt FROM @set_global_semi_cnt;
        EXECUTE set_global_semi_cnt_stmt;
        DEALLOCATE PREPARE set_global_semi_cnt_stmt;
    ELSE 
        SET GLOBAL rpl_semi_sync_source_enabled = 0;
    END IF;

END // 

DELIMITER ;