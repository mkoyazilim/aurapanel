package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	b, err := os.ReadFile("/usr/local/lsws/admin/conf/admin_config.conf")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	lines := strings.Split(string(b), "\n")
	for _, l := range lines {
		if strings.Contains(l, "address") {
			fmt.Println(l)
		}
	}
}
