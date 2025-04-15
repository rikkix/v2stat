package v2stat

import (
	"strings"
)

type TrafficDirection int

const (
	DirectionDownlink TrafficDirection = iota
	DirectionUplink
)

type ConnectionType int

const (
	ConnTypeUser ConnectionType = iota
	ConnTypeInbound
	ConnTypeOutbound
)

type ConnInfo struct {
	Type ConnectionType `json:"type"`
	Name string         `json:"name"`
}

type TrafficStat struct {
	Time     string `json:"time"`
	Downlink int64  `json:"downlink"`
	Uplink   int64  `json:"uplink"`
}

func (ci *ConnInfo) String() string {
	switch ci.Type {
	case ConnTypeUser:
		return "user:" + ci.Name
	case ConnTypeInbound:
		return "inbound:" + ci.Name
	case ConnTypeOutbound:
		return "outbound:" + ci.Name
	default:
		return "unknown:" + ci.Name
	}
}

func ParseConnInfo(s string) (ConnInfo, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return ConnInfo{}, false
	}
	var connType ConnectionType
	switch parts[0] {
	case "user":
		connType = ConnTypeUser
	case "inbound":
		connType = ConnTypeInbound
	case "outbound":
		connType = ConnTypeOutbound
	default:
		return ConnInfo{}, false
	}
	return ConnInfo{
		Type: connType,
		Name: parts[1],
	}, true
}