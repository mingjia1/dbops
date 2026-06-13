#!/bin/bash
# 使用正确密码访问41主机并传输安装包

PASS='hcfc!2017'
HOST41='10.1.81.41'
HOST16='10.1.81.16'

echo "=== Step 1: 从16获取安装包 ==="
scp root@$HOST16:/tmp/mysql_packages.tar.gz /tmp/
ls -lh /tmp/mysql_packages.tar.gz

echo ""
echo "=== Step 2: 使用密码传输到41 ==="
# 创建expect脚本
cat > /tmp/scp_to_41.exp << 'EOF'
#!/usr/bin/expect
set timeout 300
set password [lindex $argv 0]
set source [lindex $argv 1]
set dest [lindex $argv 2]

spawn scp -o StrictHostKeyChecking=no $source $dest
expect {
    "*password:" {
        send "$password\r"
        exp_continue
    }
    eof
}
EOF

chmod +x /tmp/scp_to_41.exp

# 使用expect传输
expect /tmp/scp_to_41.exp "$PASS" "/tmp/mysql_packages.tar.gz" "root@$HOST41:/opt/packet/"

echo ""
echo "=== Step 3: 在41上解压并配置 ==="
cat > /tmp/setup_41.exp << 'EOF'
#!/usr/bin/expect
set timeout 60
set password [lindex $argv 0]
set host [lindex $argv 1]
set commands [lindex $argv 2]

spawn ssh -o StrictHostKeyChecking=no root@$host
expect {
    "*password:" {
        send "$password\r"
        expect "#"
        send "$commands\r"
        expect "#"
        send "exit\r"
    }
    eof
}
EOF

chmod +x /tmp/setup_41.exp

expect /tmp/setup_41.exp "$PASS" "$HOST41" "cd /opt/packet && tar xzf mysql_packages.tar.gz && ls -lh *.deb | wc -l"

echo ""
echo "✅ 完成！"
