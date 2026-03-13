package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:20005", "CID TCP server address host:port")
	rate := flag.Int("rate", 30000, "target messages per second")
	duration := flag.Duration("duration", 20*time.Second, "test duration")
	workers := flag.Int("workers", 16, "parallel TCP workers")
	waitReply := flag.Bool("wait-reply", false, "read ACK/NACK after each message")
	deviceStart := flag.Int("device-start", 2100, "device id start")
	deviceCount := flag.Int("device-count", 8000, "number of rotating device ids")
	flag.Parse()

	if *rate <= 0 || *duration <= 0 || *workers <= 0 || *deviceCount <= 0 {
		fmt.Fprintln(os.Stderr, "invalid flags")
		os.Exit(2)
	}

	perWorkerRate := max(1, *rate/(*workers))
	interval := time.Second / time.Duration(perWorkerRate)

	var sent atomic.Int64
	var acked atomic.Int64
	var nacked atomic.Int64
	var failed atomic.Int64

	stopAt := time.Now().Add(*duration)
	var wg sync.WaitGroup
	wg.Add(*workers)

	for i := 0; i < *workers; i++ {
		go func(workerID int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", *addr)
			if err != nil {
				failed.Add(1)
				return
			}
			defer conn.Close()

			_ = conn.SetDeadline(time.Now().Add(*duration + 5*time.Second))
			reader := bufio.NewReaderSize(conn, 1024)

			nextTick := time.Now()
			seq := 0
			for time.Now().Before(stopAt) {
				seq++
				msg := makeMessage(*deviceStart+(workerID*100000+seq)%*deviceCount, seq)
				if _, err := conn.Write(msg); err != nil {
					failed.Add(1)
					return
				}
				sent.Add(1)

				if *waitReply {
					b, err := reader.ReadByte()
					if err != nil {
						failed.Add(1)
						return
					}
					switch b {
					case 0x06:
						acked.Add(1)
					case 0x15:
						nacked.Add(1)
					default:
						nacked.Add(1)
					}
				}

				nextTick = nextTick.Add(interval)
				if sleep := time.Until(nextTick); sleep > 0 {
					time.Sleep(sleep)
				}
			}
		}(i)
	}

	tStart := time.Now()
	wg.Wait()
	elapsed := time.Since(tStart)

	totalSent := sent.Load()
	rps := float64(totalSent) / elapsed.Seconds()

	fmt.Printf("target_rate=%d msg/s\n", *rate)
	fmt.Printf("workers=%d duration=%s wait_reply=%v\n", *workers, duration.String(), *waitReply)
	fmt.Printf("sent=%d failed=%d\n", totalSent, failed.Load())
	fmt.Printf("acked=%d nacked=%d\n", acked.Load(), nacked.Load())
	fmt.Printf("elapsed=%s achieved_rate=%.0f msg/s\n", elapsed.Truncate(time.Millisecond), rps)
}

func makeMessage(deviceID, seq int) []byte {
	acct := fmt.Sprintf("%04d", deviceID%10000)
	code := eventCode(seq)
	group := fmt.Sprintf("%02d", seq%99)
	zone := fmt.Sprintf("%03d", (seq%998)+1)

	// 20-byte CID payload that passes validation rules in this project.
	payload := "5000000" + acct + code + group + zone
	if len(payload) != 20 {
		panic("invalid payload len: " + strconv.Itoa(len(payload)) + " payload=" + payload)
	}
	return append([]byte(payload), 0x14)
}

func eventCode(seq int) string {
	codes := []string{"E130", "E131", "E332", "E602", "E401", "R130"}
	return strings.ToUpper(codes[seq%len(codes)])
}
