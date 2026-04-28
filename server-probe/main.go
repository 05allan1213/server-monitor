package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

var (
	cpuGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_cpu_usage_percent",
		Help: "Current CPU usage percentage",
	})
	memGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_mem_usage_percent",
		Help: "Current memory usage percentage",
	})
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	// 注册 Prometheus 指标
	prometheus.MustRegister(cpuGauge, memGauge)

	// 启动 /metrics 端点（Prometheus 来抓取）
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(":9090", nil))
	}()

	fmt.Println("🚀 系统监控Agent启动！准备建立数据库连接池...")
	// 1. 初始化数据库（从独立环境变量拼装DSN）
	host := getEnv("DB_HOST", "127.0.0.1")
	port := getEnv("DB_PORT", "3306")
	user := getEnv("DB_USER", "xiu")
	password := getEnv("DB_PASSWORD", "12345678")
	dbname := getEnv("DB_NAME", "monitor_db")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbname)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ 初始化数据库驱动失败: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Minute * 3)

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ 数据库连接失败: %v", err)
	}
	fmt.Println("✅ 数据库网络打通！准备开启 5 秒级轮询上报...")
	fmt.Println("==================================================")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	machineName, err := os.Hostname()
	if err != nil {
		machineName = "Unknown-Server"
	}
	fmt.Printf("✅ 识别到本机的物理核载名: %s\n", machineName)
	insertSql := "INSERT INTO server_metrics (ip_address, cpu_percent, mem_percent, report_time) VALUES (?, ?, ?, ?)"
	for range ticker.C {
		var usedMemPercent float64
		vMem, _ := mem.VirtualMemory()
		if vMem != nil {
			usedMemPercent = vMem.UsedPercent
		}

		var usedCpuPercent float64
		percentages, _ := cpu.Percent(time.Second, false)
		if len(percentages) > 0 {
			usedCpuPercent = percentages[0]
		}
		_, err := db.Exec(insertSql, machineName, usedCpuPercent, usedMemPercent, time.Now())

		// 更新 Prometheus 指标
		cpuGauge.Set(usedCpuPercent)
		memGauge.Set(usedMemPercent)

		currentTime := time.Now().Format("15:04:05")
		if err != nil {
			log.Printf("[%s] ❌ 上报失败: %v\n", currentTime, err)
		} else {
			fmt.Printf("[%s] 📡 成功上报 -> 内存: %.2f%% | CPU: %.2f%%\n", currentTime, usedMemPercent, usedCpuPercent)
		}
	}
}
