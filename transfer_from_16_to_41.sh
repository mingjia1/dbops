#!/bin/bash
# 在16主机上执行传输到41

echo "=== 在16主机上使用sshpass传输包到41 ==="

ssh root@10.1.81.16 << 'EOF'
PASS='hcfc!2017'

# 检查sshpass是否可用
if ! command -v sshpass &> /dev/null; then
    echo "安装sshpass..."
    apt-get update -qq && apt-get install -y sshpass
fi

echo "=== 传输包到41 ==="
sshpass -p "$PASS" scp -o StrictHostKeyChecking=no /tmp/mysql_packages.tar.gz root@10.1.81.41:/opt/packet/

echo ""
echo "=== 在41上解压 ==="
sshpass -p "$PASS" ssh -o StrictHostKeyChecking=no root@10.1.81.41 "cd /opt/packet && tar xzf mysql_packages.tar.gz && ls -lh *.deb | wc -l && echo '包文件解压完成' && ls -lh *.deb"

echo ""
echo "=== 验证41主机/opt/packet内容 ==="
sshpass -p "$PASS" ssh -o StrictHostKeyChecking=no root@10.1.81.41 "ls -lh /opt/packet/"

EOF

echo ""
echo "✅ 传输完成！"
