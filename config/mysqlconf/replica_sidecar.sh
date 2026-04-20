#!/bin/bash
###
 # @Author: Tomato
 # @Date: 2026-04-14 23:51:44
 #
 #  消费环境变量:
 #  MASTER_SERVICE: 主库的DNS记录
 #  MYSQL_ROOT_PASSWORD: 当前mysql实例root密码
 #  MYSQL_MASTER_DUMP_USER: 主从复制账号名称
 #  MYSQL_MASTER_DUMP_PASSWORD: 主从复制账号密码
### 

set -ex

# 等待mysql节点启动完成
until mysql -h 127.0.0.1 -uroot -p$MYSQL_ROOT_PASSWORD -e "SELECT 1"; do
    echo "Waiting for mysql to be ready"
    sleep 5
done

# 获取bin-log备份信息
# https://docs.percona.com/percona-xtrabackup/8.4/create-gtid-replica.html
cd /var/lib/mysql

executed_gtid_set=$(cat xtrabackup_binlog_info | awk '{print $NF}')

# 启动从节点复制
mysql -h 127.0.0.1 -uroot -p$MYSQL_ROOT_PASSWORD -e "$(</mnt/configmap/start_replication_procedure.sql)"
mysql -h 127.0.0.1 -uroot -p$MYSQL_ROOT_PASSWORD -e "call mysql.StartReplication('$MASTER_SERVICE', '$MYSQL_MASTER_DUMP_USER', '$MYSQL_MASTER_DUMP_PASSWORD','$executed_gtid_set')"

# 启动xtrabackup
exec ncat --listen --keep-open --send-only --max-conns=1 3307 -c \
"xtrabackup --backup --slave-info --stream=xbstream --host=127.0.0.1 --user=root --password=$MYSQL_ROOT_PASSWORD"