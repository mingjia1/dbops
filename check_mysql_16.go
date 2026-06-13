package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
)

func main() {
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("hcfc!2017")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, _ := ssh.Dial("tcp", "10.1.81.16:22", config)
	defer client.Close()

	commands := []string{
		// 查看运行的MySQL实例
		"ss -tlnp | grep mysql || netstat -tlnp 2>/dev/null | grep mysql",
		"ps aux | grep mysqld | grep -v grep | awk '{print $2}' | head -3",
		// 查看MySQL端口
		"ss -tlnp | grep -E ':3306|:3307|:3308' ",
		// 尝试无密码连接
		"mysql -h127.0.0.1 -P3306 -uroot -e 'SELECT VERSION()' 2>&1 | head -3",
		// 检查是否有.my.cnf
		"cat /root/.my.cnf 2>/dev/null | head -5 || echo 'No .my.cnf'",
		// 查看datadir和配置
		"ls /data/ 2>/dev/null; ls /etc/mysql/ 2>/dev/null | head",
	}

	for i, cmd := range commands {
		fmt.Printf("\n[%d] %s\n", i+1, cmd)
		session, _ := client.NewSession()
		output, _ := session.CombinedOutput(cmd)
		session.Close()
		fmt.Printf("%s\n", string(output))
	}
}
