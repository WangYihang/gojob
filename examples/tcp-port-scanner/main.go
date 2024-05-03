package main

import (
	"github.com/WangYihang/gojob"
)

func main() {
	var commonPorts = []uint16{
		21, 22, 23, 25, 53, 80, 110, 135, 139, 143, 443, 445, 993, 995,
		1723, 3306, 3389, 5900, 8080,
	}
	var ips = []string{
		"192.168.200.1",
		"192.168.200.254",
	}
	scheduler := gojob.New(
		gojob.WithNumWorkers(8),
		gojob.WithMaxRetries(4),
		gojob.WithMaxRuntimePerTaskSeconds(16),
		gojob.WithNumShards(1),
		gojob.WithShard(0),
		gojob.WithTotalTasks(int64(len(ips)*len(commonPorts))),
		gojob.WithStatusFilePath("status.json"),
		gojob.WithResultFilePath("result.json"),
		gojob.WithMetadataFilePath("metadata.json"),
	).
		Start()
	for _, ip := range ips {
		for _, port := range commonPorts {
			scheduler.Submit(New(ip, port))
		}
	}
	scheduler.Wait()
}
