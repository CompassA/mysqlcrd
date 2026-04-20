#!/bin/bash
###
 # @Author: Tomato
 # @Date: 2026-04-14 23:14:28
 #
 # 消费环境变量:
 # PODNAME: pod名称
 # POD_NAMESPACE: pod所属的namespace
 # MASTER_SERVICE: 主库的DNS记录
 # REPLICA_SERVICE_NAME: 从库服务名称
### 

set -ex

# 获取POD序号
ORDINAL=$(echo $PODNAME | awk -F'-' '{print $NF}')

# 生成从库的server-id与配置文件
echo [mysqld] > /etc/mysql/conf.d/server-id.cnf
echo server-id=$((100 + $ORDINAL)) >> /etc/mysql/conf.d/server-id.cnf
cp /mnt/configmap/replica.cnf /etc/mysql/conf.d/my.cnf

# 全量拷贝
## 这个文件夹存在时, 说明执行过拷贝, 不再执行后续的操作
if [[ -d /var/lib/mysql/mysql ]]; then
    echo "clone had been executed, skip"
    exit 0
fi

## 获取被拷贝服务
SOURCE_URL=$MASTER_SERVICE

# 获取POD序号
# 是0代表是第一个从节点, 从主节点拷贝数据, 否则从前一个从节点拷贝数据
if [[ $ORDINAL -ne 0 ]]; then
    PODNAME_PREFIX=$(echo $PODNAME | rev | cut -d- -f2- | rev)
    SOURCE_URL=$PODNAME_PREFIX-$(($ORDINAL-1)).$REPLICA_SERVICE_NAME.$POD_NAMESPACE
fi

# 执行拷贝
ncat --recv-only $SOURCE_URL 3307 | xbstream -x -C /var/lib/mysql
xtrabackup --prepare --target-dir=/var/lib/mysql