package main

import (
	"context"
	"net"
	"strconv"
)

type MyTask struct {
	IP     string `json:"ip"`
	Port   uint16 `json:"port"`
	Status Status `json:"status"`
}

func New(ip string, port uint16) *MyTask {
	return &MyTask{
		IP:     ip,
		Port:   port,
		Status: Unknown,
	}
}

func (t *MyTask) Do(ctx context.Context) error {
	var dialer net.Dialer
	address := net.JoinHostPort(t.IP, strconv.Itoa(int(t.Port)))
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		t.Status = Closed
		return nil
	}
	defer conn.Close()
	t.Status = Open
	return nil
}
