USE mysql;

delimiter //

-- source_host        主库url
-- username           主从复制账号名称
-- userpassword       主从复制账号密码
-- executed_gtid_set  xtrabackup中获取的已执行的gtid集合
CREATE PROCEDURE IF NOT EXISTS StartReplication(IN source_host VARCHAR(4096),IN username VARCHAR(512),IN userpassword VARCHAR(512),IN executed_gtid_set VARCHAR(1024))
BEGIN
    DECLARE config_cnt INT;
    DECLARE replica_io_running VARCHAR(32);
    DECLARE cur_host VARCHAR(4096);
    DECLARE replica_sql_running VARCHAR(32);
    
    -- 查询是否执行过 change replication source
    -- https://dev.mysql.com/doc/refman/8.4/en/performance-schema-replication-connection-configuration-table.html
    SELECT COUNT(*) INTO config_cnt FROM performance_schema.replication_connection_configuration WHERE channel_name = '';
    IF config_cnt = 0 THEN
       
        SET @@GLOBAL.GTID_PURGED=executed_gtid_set;

        SET @change_replica_sql = CONCAT(
            'change replication source to ',
            'SOURCE_HOST=''', source_host, ''',',
            'SOURCE_USER=''', username, ''',',
            'SOURCE_PASSWORD=''', userpassword, ''',',
            'SOURCE_PORT=3306, GET_SOURCE_PUBLIC_KEY=1, SOURCE_AUTO_POSITION=1'
        );
        PREPARE change_replica_stmt FROM @change_replica_sql;
        EXECUTE change_replica_stmt;
        DEALLOCATE PREPARE change_replica_stmt;

        SELECT CONCAT('change replication source to ', source_host, ' success') AS message;
    ELSE 
        SELECT HOST INTO cur_host FROM performance_schema.replication_connection_configuration WHERE channel_name = '';
        IF cur_host != source_host THEN    
            SET @@GLOBAL.GTID_PURGED=executed_gtid_set;

            SET @change_replica_sql = CONCAT(
                'change replication source to ',
                'SOURCE_HOST=''', source_host, ''',',
                'SOURCE_USER=''', username, ''',',
                'SOURCE_PASSWORD=''', userpassword, ''',',
                'SOURCE_PORT=3306, GET_SOURCE_PUBLIC_KEY=1, SOURCE_AUTO_POSITION=1'
            );
            PREPARE change_replica_stmt FROM @change_replica_sql;
            EXECUTE change_replica_stmt;
            DEALLOCATE PREPARE change_replica_stmt;

            SELECT CONCAT('change replication source to ', source_host, ' success') AS message; 
        ELSE 
            SELECT CONCAT('change replication source to ', source_host, ' already executed') AS message;
        END IF;
        
    END IF;

    -- 查询I/O thread, reply thread状态, 判断主从是否开启
    -- LAST_ERROR_MESSAGE
    -- https://dev.mysql.com/doc/refman/8.4/en/performance-schema-replication-connection-status-table.html
    -- https://dev.mysql.com/doc/refman/8.4/en/performance-schema-replication-applier-status-table.html
    SELECT service_state INTO replica_io_running FROM performance_schema.replication_connection_status WHERE channel_name = '';
    SELECT service_state INTO replica_sql_running FROM performance_schema.replication_applier_status WHERE channel_name = '';

    IF replica_io_running != 'ON' OR replica_sql_running != 'ON' THEN
        start replica;
    ELSE
        SELECT 'replica already started' AS message;
    END IF;
    
    show replica status;
END // 

DELIMITER ;