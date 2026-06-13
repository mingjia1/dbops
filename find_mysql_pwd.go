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

	// 尝试常见密码连接3306
	passwords := []string{"hcfc!2017", "root", "123456", "password", "Root@123", "mysql"}

	for _, pwd := range passwords {
		session, _ := client.NewSession()
		cmd := fmt.Sprintf("MYSQL_PWD='%s' mysql -h127.0.0.1 -P3306 -uroot -e 'SELECT 1' 2>&1 | head -1", pwd)
		out, _ := session.CombinedOutput(cmd)
		session.Close()

		result := string(out)
		if len(result) > 0 && result[0] == '1' {
			fmt.Printf("✅ Password found: '%s'\n", pwd)
			return
		} else {
			fmt.Printf("❌ '%s': %s", pwd, result)
		}
	}

	// 查看mysql配置文件中可能的密码
	fmt.Println("\n=== Checking config files ===")
	session, _ := client.NewSession()
	out, _ := session.CombinedOutput("grep -r 'password' /etc/mysql/ 2>/dev/null | grep -v '#' | head -5; find / -name '.my.cnf' 2>/dev/null | head -3")
	session.Close()
	fmt.Printf("%s\n", string(out))
}
