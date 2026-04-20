#!/bin/bash
###
 # @Author: Tomato
 # @Date: 2026-04-14 02:25:23
 
 # 消费环境变量:
 ## MYSQL_ROOT_PASSWORD:          mysql root密码
 ## MYSQL_MASTER_DUMP_USER:       mysql 主从复制账号
 ## MYSQL_MASTER_DUMP_PASSWORD:   mysql 主从复制账号密码
### 

# 任意一条命令执行失败（返回非零状态码）时，‌立即退出脚本
# 在执行每条命令前，‌将该命令及其参数打印到标准错误输出
set -ex

# 等待mysql节点启动完成
until mysql -h 127.0.0.1 -uroot -p$MYSQL_ROOT_PASSWORD -e "SELECT 1"; do
echo "Waiting for mysql to be ready"
sleep 5
done

# 创建主从复制账号与密码
mysql -h 127.0.0.1 -uroot -p$MYSQL_ROOT_PASSWORD -e "$(</mnt/configmap/create_replication_account_procedure.sql)"
mysql -h 127.0.0.1 -uroot -p$MYSQL_ROOT_PASSWORD -e "call mysql.CreateDumpUser('$MYSQL_MASTER_DUMP_USER', '$MYSQL_MASTER_DUMP_PASSWORD')"
echo "mysql-master sidecar-task finished"

# 启动xtrabackup监听
exec ncat --listen --keep-open --send-only --max-conns=1 3307 -c \
"xtrabackup --backup --slave-info --stream=xbstream --host=127.0.0.1 --user=root --password=$MYSQL_ROOT_PASSWORD"