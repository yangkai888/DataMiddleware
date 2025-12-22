package test

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestSystemParameters 测试系统参数调优
func TestSystemParameters(t *testing.T) {
	t.Run("NetworkParameters", func(t *testing.T) {
		testNetworkParameters(t)
	})

	t.Run("FileDescriptors", func(t *testing.T) {
		testFileDescriptorLimits(t)
	})

	t.Run("KernelParameters", func(t *testing.T) {
		testKernelParameters(t)
	})

	t.Run("TCPStackOptimization", func(t *testing.T) {
		testTCPStackOptimization(t)
	})
}

// testNetworkParameters 测试网络参数
func testNetworkParameters(t *testing.T) {
	t.Run("TCPKeepAlive", func(t *testing.T) {
		// 检查TCP keepalive设置
		if value := getSysctlValue("net.ipv4.tcp_keepalive_time"); value != "" {
			if intVal, err := strconv.Atoi(value); err == nil {
				t.Logf("TCP keepalive时间: %d秒", intVal)
				if intVal > 7200 { // 2小时
					t.Logf("建议: TCP keepalive时间较长 (%d秒)，可能影响连接回收", intVal)
				} else {
					t.Logf("✓ TCP keepalive时间合理: %d秒", intVal)
				}
			}
		}
	})

	t.Run("TCPMaxSynBacklog", func(t *testing.T) {
		// 检查TCP SYN队列长度
		if value := getSysctlValue("net.ipv4.tcp_max_syn_backlog"); value != "" {
			if intVal, err := strconv.Atoi(value); err == nil {
				t.Logf("TCP最大SYN队列: %d", intVal)
				if intVal < 1024 {
					t.Logf("建议: TCP SYN队列可能过小 (%d)，高并发下可能丢弃连接", intVal)
				} else {
					t.Logf("✓ TCP SYN队列设置合理: %d", intVal)
				}
			}
		}
	})

	t.Run("TCPTwReuse", func(t *testing.T) {
		// 检查TIME_WAIT重用
		if value := getSysctlValue("net.ipv4.tcp_tw_reuse"); value != "" {
			if intVal, err := strconv.Atoi(value); err == nil {
				if intVal == 1 {
					t.Logf("✓ TCP TIME_WAIT重用已启用")
				} else {
					t.Logf("建议: 启用TCP TIME_WAIT重用以减少TIME_WAIT连接")
				}
			}
		}
	})

	t.Run("LocalPortRange", func(t *testing.T) {
		// 检查本地端口范围
		if value := getProcValue("/proc/sys/net/ipv4/ip_local_port_range"); value != "" {
			parts := strings.Fields(value)
			if len(parts) == 2 {
				minPort, err1 := strconv.Atoi(parts[0])
				maxPort, err2 := strconv.Atoi(parts[1])
				if err1 == nil && err2 == nil {
					portRange := maxPort - minPort + 1
					t.Logf("本地端口范围: %d-%d (总共%d个端口)", minPort, maxPort, portRange)

					if portRange < 10000 {
						t.Logf("警告: 本地端口范围较小 (%d)，高并发下可能端口耗尽", portRange)
					} else {
						t.Logf("✓ 本地端口范围充足: %d个端口", portRange)
					}
				}
			}
		}
	})
}

// testFileDescriptorLimits 测试文件描述符限制
func testFileDescriptorLimits(t *testing.T) {
	t.Run("ProcessLimits", func(t *testing.T) {
		// 检查进程文件描述符限制
		if value := getProcValue("/proc/sys/fs/file-max"); value != "" {
			if intVal, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				t.Logf("系统最大文件描述符: %d", intVal)

				// 检查进程限制
				cmd := exec.Command("bash", "-c", "ulimit -n")
				if output, err := cmd.Output(); err == nil {
					if processLimit, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil {
						t.Logf("进程文件描述符限制: %d", processLimit)

						if processLimit < 1024 {
							t.Logf("警告: 进程文件描述符限制过低 (%d)", processLimit)
						} else if processLimit >= intVal {
							t.Logf("✓ 进程文件描述符限制合理: %d", processLimit)
						} else {
							t.Logf("进程限制 (%d) 低于系统最大值 (%d)", processLimit, intVal)
						}
					}
				}
			}
		}
	})

	t.Run("OpenFileCount", func(t *testing.T) {
		// 检查当前打开的文件数
		if value := getProcValue("/proc/sys/fs/file-nr"); value != "" {
			parts := strings.Fields(value)
			if len(parts) >= 1 {
				if openFiles, err := strconv.Atoi(parts[0]); err == nil {
					t.Logf("当前打开的文件数: %d", openFiles)

					// 这是一个基本检查，实际应该与系统容量比较
					if openFiles > 10000 {
						t.Logf("✓ 系统文件打开数正常: %d", openFiles)
					} else {
						t.Logf("系统文件打开数: %d (正常范围)", openFiles)
					}
				}
			}
		}
	})
}

// testKernelParameters 测试内核参数
func testKernelParameters(t *testing.T) {
	t.Run("Somaxconn", func(t *testing.T) {
		// 检查监听队列最大长度
		if value := getSysctlValue("net.core.somaxconn"); value != "" {
			if intVal, err := strconv.Atoi(value); err == nil {
				t.Logf("Socket监听队列最大长度: %d", intVal)

				if intVal < 1024 {
					t.Logf("建议: 监听队列长度较小 (%d)，高并发下可能拒绝连接", intVal)
				} else {
					t.Logf("✓ 监听队列长度合理: %d", intVal)
				}
			}
		}
	})

	t.Run("NetdevMaxBacklog", func(t *testing.T) {
		// 检查网络设备队列长度
		if value := getSysctlValue("net.core.netdev_max_backlog"); value != "" {
			if intVal, err := strconv.Atoi(value); err == nil {
				t.Logf("网络设备最大队列: %d", intVal)

				if intVal < 1000 {
					t.Logf("建议: 网络设备队列可能过小 (%d)", intVal)
				} else {
					t.Logf("✓ 网络设备队列设置合理: %d", intVal)
				}
			}
		}
	})

	t.Run("RmemMax", func(t *testing.T) {
		// 检查TCP接收缓冲区最大值
		if value := getSysctlValue("net.core.rmem_max"); value != "" {
			if intVal, err := strconv.Atoi(value); err == nil {
				rmemMaxKB := intVal / 1024
				t.Logf("TCP接收缓冲区最大值: %d KB", rmemMaxKB)

				if rmemMaxKB < 4096 { // 4MB
					t.Logf("建议: TCP接收缓冲区较小 (%d KB)，可能影响高带宽传输", rmemMaxKB)
				} else {
					t.Logf("✓ TCP接收缓冲区设置合理: %d KB", rmemMaxKB)
				}
			}
		}
	})

	t.Run("WmemMax", func(t *testing.T) {
		// 检查TCP发送缓冲区最大值
		if value := getSysctlValue("net.core.wmem_max"); value != "" {
			if intVal, err := strconv.Atoi(value); err == nil {
				wmemMaxKB := intVal / 1024
				t.Logf("TCP发送缓冲区最大值: %d KB", wmemMaxKB)

				if wmemMaxKB < 4096 { // 4MB
					t.Logf("建议: TCP发送缓冲区较小 (%d KB)，可能影响高带宽传输", wmemMaxKB)
				} else {
					t.Logf("✓ TCP发送缓冲区设置合理: %d KB", wmemMaxKB)
				}
			}
		}
	})
}

// testTCPStackOptimization 测试TCP协议栈优化
func testTCPStackOptimization(t *testing.T) {
	t.Run("TCPFastOpen", func(t *testing.T) {
		// 检查TCP快速打开
		if value := getSysctlValue("net.ipv4.tcp_fastopen"); value != "" {
			if intVal, err := strconv.Atoi(value); err == nil {
				if intVal > 0 {
					t.Logf("✓ TCP快速打开已启用 (level %d)", intVal)
				} else {
					t.Logf("TCP快速打开未启用")
				}
			}
		}
	})

	t.Run("TCPTimestamps", func(t *testing.T) {
		// 检查TCP时间戳
		if value := getSysctlValue("net.ipv4.tcp_timestamps"); value != "" {
			if intVal, err := strconv.Atoi(value); err == nil {
				if intVal == 1 {
					t.Logf("✓ TCP时间戳已启用")
				} else {
					t.Logf("TCP时间戳未启用")
				}
			}
		}
	})

	t.Run("TCPSack", func(t *testing.T) {
		// 检查TCP SACK
		if value := getSysctlValue("net.ipv4.tcp_sack"); value != "" {
			if intVal, err := strconv.Atoi(value); err == nil {
				if intVal == 1 {
					t.Logf("✓ TCP SACK已启用")
				} else {
					t.Logf("TCP SACK未启用")
				}
			}
		}
	})

	t.Run("TCPWindowScaling", func(t *testing.T) {
		// 检查TCP窗口缩放
		if value := getSysctlValue("net.ipv4.tcp_window_scaling"); value != "" {
			if intVal, err := strconv.Atoi(value); err == nil {
				if intVal == 1 {
					t.Logf("✓ TCP窗口缩放已启用")
				} else {
					t.Logf("TCP窗口缩放未启用")
				}
			}
		}
	})
}

// TestNetworkConnectivity 测试网络连接性
func TestNetworkConnectivity(t *testing.T) {
	t.Run("DNSResolution", func(t *testing.T) {
		// 测试DNS解析
		start := time.Now()
		_, err := net.LookupHost("google.com")
		duration := time.Since(start)

		if err != nil {
			t.Logf("DNS解析失败: %v", err)
		} else {
			t.Logf("✓ DNS解析成功，耗时: %v", duration)
		}
	})

	t.Run("ExternalConnectivity", func(t *testing.T) {
		// 测试外部连接性
		conn, err := net.DialTimeout("tcp", "google.com:80", 5*time.Second)
		if err != nil {
			t.Logf("外部连接测试失败: %v", err)
		} else {
			conn.Close()
			t.Logf("✓ 外部网络连接正常")
		}
	})

	t.Run("LocalhostConnectivity", func(t *testing.T) {
		// 测试本地连接
		conn, err := net.DialTimeout("tcp", "localhost:9090", 1*time.Second)
		if err != nil {
			t.Logf("本地服务连接失败 (正常，如果服务未启动): %v", err)
		} else {
			conn.Close()
			t.Logf("✓ 本地服务连接正常")
		}
	})
}

// TestSystemLoad 测试系统负载
func TestSystemLoad(t *testing.T) {
	t.Run("LoadAverage", func(t *testing.T) {
		// 检查系统负载
		if loadAvg := getProcValue("/proc/loadavg"); loadAvg != "" {
			parts := strings.Fields(loadAvg)
			if len(parts) >= 3 {
				load1, err1 := strconv.ParseFloat(parts[0], 64)
				load5, err2 := strconv.ParseFloat(parts[1], 64)
				load15, err3 := strconv.ParseFloat(parts[2], 64)

				if err1 == nil && err2 == nil && err3 == nil {
					t.Logf("系统负载: 1分钟=%.2f, 5分钟=%.2f, 15分钟=%.2f", load1, load5, load15)

					// 检查负载是否过高 (假设2个CPU核心)
					const cpuCores = 2.0
					if load1 > cpuCores*1.5 {
						t.Logf("警告: 1分钟平均负载较高 (%.2f > %.1f)", load1, cpuCores*1.5)
					} else {
						t.Logf("✓ 系统负载正常")
					}
				}
			}
		}
	})

	t.Run("MemoryUsage", func(t *testing.T) {
		// 检查内存使用情况
		if memInfo := getProcValue("/proc/meminfo"); memInfo != "" {
			// 解析内存信息
			scanner := bufio.NewScanner(strings.NewReader(memInfo))
			memTotal := int64(0)
			memAvailable := int64(0)

			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "MemTotal:") {
					if val := extractMemValue(line); val > 0 {
						memTotal = val
					}
				} else if strings.HasPrefix(line, "MemAvailable:") {
					if val := extractMemValue(line); val > 0 {
						memAvailable = val
					}
				}
			}

			if memTotal > 0 && memAvailable > 0 {
				memUsed := memTotal - memAvailable
				memUsagePercent := float64(memUsed) / float64(memTotal) * 100

				t.Logf("内存使用: %d/%d KB (%.1f%%)",
					memUsed/1024, memTotal/1024, memUsagePercent)

				if memUsagePercent > 90 {
					t.Logf("警告: 内存使用率过高 (%.1f%%)", memUsagePercent)
				} else {
					t.Logf("✓ 内存使用正常")
				}
			}
		}
	})
}

// 辅助函数

// getSysctlValue 获取sysctl值
func getSysctlValue(key string) string {
	cmd := exec.Command("sysctl", "-n", key)
	if output, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(output))
	}
	return ""
}

// getProcValue 读取/proc文件
func getProcValue(path string) string {
	if data, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// extractMemValue 从内存信息行提取数值
func extractMemValue(line string) int64 {
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		if val, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			return val
		}
	}
	return 0
}

// BenchmarkNetworkLatency 网络延迟基准测试
func BenchmarkNetworkLatency(b *testing.B) {
	// 创建一个简单的echo服务器用于测试
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		b.Skip("无法创建测试服务器")
		return
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					c.Write(buf[:n])
				}
			}(conn)
		}
	}()

	addr := listener.Addr().String()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			b.Fatal(err)
		}
		defer conn.Close()

		buf := make([]byte, 64)
		for pb.Next() {
			conn.Write(buf)
			conn.Read(buf)
		}
	})
}
