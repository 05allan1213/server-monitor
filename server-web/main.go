package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 1. 数据结构体（与探针保持完全一致，用来读取数据）
// 1. 数据结构体 (用来读取数据并传给前端)
type ServerStatus struct { // 找到这块代码
	ID          uint      `gorm:"primaryKey"`
	IPAddress   string    `gorm:"column:ip_address"`
	CPUUsage    float64   `gorm:"column:cpu_percent"`
	MemoryUsage float64   `gorm:"column:mem_percent"`
	ReportTime  time.Time `gorm:"column:report_time"`
}

// 确保你有这个方法绑定表名（如果没有可以不加，如果原本有就保留）
func (ServerStatus) TableName() string {
	return "server_metrics"
}

var db *gorm.DB

func initDB() {
	host := getEnv("DB_HOST", "192.168.106.132")
	port := getEnv("DB_PORT", "3306")
	user := getEnv("DB_USER", "monody")
	password := getEnv("DB_PASSWORD", "12345678")
	dbname := getEnv("DB_NAME", "monitor_db")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbname)

	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败，报错信息：", err)
	}

	db.AutoMigrate(&ServerStatus{})
	db.Exec("ALTER TABLE server_metrics MODIFY COLUMN report_time DATETIME DEFAULT CURRENT_TIMESTAMP")
	fmt.Println("✅ 数据库表结构自动同步完成！")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	// 连接数据库
	initDB()

	// 开启一个 Gin Web 引擎
	r := gin.Default()

	// 写一个直接输出 HTML 前端页面的路由 (极致硬核：Go语言直出HTML)
	r.GET("/", func(c *gin.Context) {
		var stats []ServerStatus
		// 倒序查询最近上报的 10 条数据
		// 不按时间排了！直接让 MySQL 给我们吐出 10 条数据！
		// 加上 Order("id desc")，按最新写入的倒序拿 10 条！
		db.Order("id desc").Limit(10).Find(&stats)

		fmt.Println("\n====== 🚨 透视眼拦截到的数据 🚨 ======")
		fmt.Printf("%+v\n", stats)
		fmt.Println("======================================")
		// 动态拼装一段极简风格的 HTML 表格页面
		// 重点是 head 里面的 meta 标签，数字 2 代表每 2 秒自动刷新！
		html := `<html>
		<head>
			<meta charset="utf-8">
			<meta http-equiv="refresh" content="2"> 
			<title>超引力 - 监控大盘</title>
		</head>
		<body style="font-family: Arial; padding: 20px;">
			<h2>🚀 大盘控制中心：全服实时监控状态</h2>
			<table border="1" style="width: 100%; text-align: center; border-collapse: collapse;">
				<tr style="background-color: #4CAF50; color: white;">
					<th>主机名/IP地址</th>
					<th>CPU 使用率</th>
					<th>内存 使用率</th>
					<th>最后上报时间</th>
				</tr>`

		// 循环把数据库的数据填入表格当中
		for _, s := range stats {
			html += fmt.Sprintf(`
			<tr>
				<td>%s</td>
				<td><b style="color:red">%.2f%%</b></td>
				<td><b style="color:blue">%.2f%%</b></td>
				<td>%s</td> 
			</tr>`, s.IPAddress, s.CPUUsage, s.MemoryUsage, s.ReportTime.Format("2006-01-02 15:04:05")) // 👈 重点是这最后的格式化
		}
		html += `</table></body></html>`

		// 返回 HTML 内容给浏览器
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})

	// 在 8080 端口启动 Web 服务器
	fmt.Println("🎉 Web 服务启动成功！请打开浏览器访问：http://localhost:8080")
	r.Run(":8080")
}
