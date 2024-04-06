package model

import "fmt"

type RouteInfo struct {
	Destination uint32
	Mask        uint32
	Gateway     uint32
}

func (r *RouteInfo) GetDst() string {
	return fmt.Sprintf("%d.%d.%d.%d", r.Destination>>24, (r.Destination>>16)&0xFF, (r.Destination>>8)&0xFF, r.Destination&0xFF)
}
func (r *RouteInfo) GetMask() string {
	return fmt.Sprintf("%d.%d.%d.%d", r.Mask>>24, (r.Mask>>16)&0xFF, (r.Mask>>8)&0xFF, r.Mask&0xFF)
}
func (r *RouteInfo) GetGw() string {
	return fmt.Sprintf("%d.%d.%d.%d", r.Gateway>>24, (r.Gateway>>16)&0xFF, (r.Gateway>>8)&0xFF, r.Gateway&0xFF)
}
