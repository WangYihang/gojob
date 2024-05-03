package main

import "net"

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

func (t *MyTask) Do() error {
	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{
		IP:   net.ParseIP(t.IP),
		Port: int(t.Port),
	})
	if err != nil {
		t.Status = Closed
		return nil
	}
	defer conn.Close()
	t.Status = Open
	return nil
}
