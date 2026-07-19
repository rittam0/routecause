//go:build windows

package platform

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"
)

var (
	wlanDLL            = syscall.NewLazyDLL("wlanapi.dll")
	wlanOpenHandle     = wlanDLL.NewProc("WlanOpenHandle")
	wlanCloseHandle    = wlanDLL.NewProc("WlanCloseHandle")
	wlanEnumInterfaces = wlanDLL.NewProc("WlanEnumInterfaces")
	wlanQueryInterface = wlanDLL.NewProc("WlanQueryInterface")
	wlanFreeMemory     = wlanDLL.NewProc("WlanFreeMemory")
	iphlpDLL           = syscall.NewLazyDLL("iphlpapi.dll")
	getIPForwardTable2 = iphlpDLL.NewProc("GetIpForwardTable2")
	freeMibTable       = iphlpDLL.NewProc("FreeMibTable")
	getIfEntry2        = iphlpDLL.NewProc("GetIfEntry2")
	icmpCreateFile     = iphlpDLL.NewProc("IcmpCreateFile")
	icmpCloseHandle    = iphlpDLL.NewProc("IcmpCloseHandle")
	icmpSendEcho       = iphlpDLL.NewProc("IcmpSendEcho")
)

const (
	afINET                          = 2
	wlanStateConnected              = 1
	wlanIntfOpcodeCurrentConnection = 7
	errorSuccess                    = 0
	invalidHandle                   = ^uintptr(0)
)

type winGUID struct {
	Data1        uint32
	Data2, Data3 uint16
	Data4        [8]byte
}
type wlanInterfaceInfo struct {
	GUID        winGUID
	Description [256]uint16
	State       uint32
}
type wlanInterfaceList struct {
	Count uint32
	Index uint32
	First wlanInterfaceInfo
}
type dot11SSID struct {
	Length uint32
	Value  [32]byte
}
type wlanAssociation struct {
	SSID          dot11SSID
	BSSType       uint32
	BSSID         [6]byte
	Padding       [2]byte
	PHYType       uint32
	PHYIndex      uint32
	SignalQuality uint32
	ReceiveRate   uint32
	TransmitRate  uint32
}
type wlanConnection struct {
	State       uint32
	Mode        uint32
	Profile     [256]uint16
	Association wlanAssociation
}

type windowsWiFiAPI struct{}

func (windowsWiFiAPI) Snapshot(ctx context.Context) (WiFiSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return WiFiSnapshot{}, err
	}
	var negotiated uint32
	var handle uintptr
	r, _, _ := wlanOpenHandle.Call(2, 0, uintptr(unsafe.Pointer(&negotiated)), uintptr(unsafe.Pointer(&handle)))
	if r != errorSuccess {
		return WiFiSnapshot{}, fmt.Errorf("WlanOpenHandle: status %d", r)
	}
	defer wlanCloseHandle.Call(handle, 0)
	var listPtr unsafe.Pointer
	r, _, _ = wlanEnumInterfaces.Call(handle, 0, uintptr(unsafe.Pointer(&listPtr)))
	if r != errorSuccess {
		return WiFiSnapshot{}, fmt.Errorf("WlanEnumInterfaces: status %d", r)
	}
	if listPtr == nil {
		return WiFiSnapshot{}, ErrNoWiFiAdapter
	}
	defer wlanFreeMemory.Call(uintptr(listPtr))
	list := (*wlanInterfaceList)(listPtr)
	if list.Count == 0 {
		return WiFiSnapshot{}, ErrNoWiFiAdapter
	}
	items := unsafe.Slice(&list.First, int(list.Count))
	connectedCount := 0
	for i := range items {
		if items[i].State == wlanStateConnected {
			connectedCount++
		}
	}
	if connectedCount > 1 {
		return WiFiSnapshot{}, ErrRouteAmbiguous
	}
	for i := range items {
		if items[i].State != wlanStateConnected {
			continue
		}
		var dataSize, opcodeType uint32
		var dataPtr unsafe.Pointer
		r, _, _ = wlanQueryInterface.Call(handle, uintptr(unsafe.Pointer(&items[i].GUID)), wlanIntfOpcodeCurrentConnection, 0, uintptr(unsafe.Pointer(&dataSize)), uintptr(unsafe.Pointer(&dataPtr)), uintptr(unsafe.Pointer(&opcodeType)))
		if r != errorSuccess || dataPtr == nil {
			return WiFiSnapshot{}, fmt.Errorf("WlanQueryInterface: status %d", r)
		}
		defer wlanFreeMemory.Call(uintptr(dataPtr))
		connection := (*wlanConnection)(dataPtr)
		length := connection.Association.SSID.Length
		if length > uint32(len(connection.Association.SSID.Value)) {
			length = uint32(len(connection.Association.SSID.Value))
		}
		return WiFiSnapshot{AdapterPresent: true, Connected: true, SSID: string(connection.Association.SSID.Value[:length]), SignalQuality: connection.Association.SignalQuality, ReceiveKbps: connection.Association.ReceiveRate, TransmitKbps: connection.Association.TransmitRate, Radio: phyName(connection.Association.PHYType)}, nil
	}
	return WiFiSnapshot{AdapterPresent: true, Connected: false}, nil
}

func phyName(value uint32) string {
	switch value {
	case 4:
		return "802.11a"
	case 5:
		return "802.11b"
	case 6:
		return "802.11g"
	case 7:
		return "802.11n"
	case 8:
		return "802.11ac"
	case 10:
		return "802.11ax"
	default:
		return "unknown"
	}
}

type windowsRouteAPI struct{}

func (windowsRouteAPI) DefaultIPv4Route(ctx context.Context) (DefaultRoute, error) {
	if err := ctx.Err(); err != nil {
		return DefaultRoute{}, err
	}
	var table unsafe.Pointer
	r, _, _ := getIPForwardTable2.Call(afINET, uintptr(unsafe.Pointer(&table)))
	if r != errorSuccess {
		return DefaultRoute{}, fmt.Errorf("GetIpForwardTable2: status %d", r)
	}
	if table == nil {
		return DefaultRoute{}, ErrNoDefaultRoute
	}
	defer freeMibTable.Call(uintptr(table))
	count := *(*uint32)(table)
	const rowSize, rowsOffset = uintptr(104), uintptr(8)
	var best *DefaultRoute
	for i := uint32(0); i < count; i++ {
		row := unsafe.Slice((*byte)(unsafe.Add(table, rowsOffset+uintptr(i)*rowSize)), int(rowSize))
		if binary.LittleEndian.Uint16(row[12:14]) != afINET || row[40] != 0 {
			continue
		}
		if binary.LittleEndian.Uint16(row[44:46]) != afINET {
			continue
		}
		gateway := net.IPv4(row[48], row[49], row[50], row[51])
		if gateway.Equal(net.IPv4zero) {
			continue
		}
		candidate := DefaultRoute{Gateway: gateway, InterfaceIndex: binary.LittleEndian.Uint32(row[8:12]), Metric: binary.LittleEndian.Uint32(row[84:88])}
		candidate.Kind = interfaceKind(candidate.InterfaceIndex)
		if best == nil || candidate.Metric < best.Metric {
			copy := candidate
			best = &copy
		} else if candidate.Metric == best.Metric && candidate.InterfaceIndex != best.InterfaceIndex {
			best.Ambiguous = true
		}
	}
	if best == nil {
		return DefaultRoute{}, ErrNoDefaultRoute
	}
	if best.Ambiguous {
		return *best, ErrRouteAmbiguous
	}
	return *best, nil
}

func interfaceKind(index uint32) ConnectionKind {
	row := make([]byte, 1408)
	binary.LittleEndian.PutUint32(row[8:12], index)
	r, _, _ := getIfEntry2.Call(uintptr(unsafe.Pointer(&row[0])))
	if r != errorSuccess {
		return ConnectionUnknown
	}
	// MIB_IF_ROW2.Type follows LUID, index, GUID, two WCHAR[257]
	// strings, physical-address length/arrays, and MTU.
	switch binary.LittleEndian.Uint32(row[1128:1132]) {
	case 6:
		return ConnectionEthernet
	case 71:
		return ConnectionWiFi
	case 53, 131:
		return ConnectionVPN
	default:
		return ConnectionUnknown
	}
}

type windowsEchoAPI struct{}

func (windowsEchoAPI) Echo(ctx context.Context, address net.IP, timeout time.Duration) (EchoReply, error) {
	if err := ctx.Err(); err != nil {
		return EchoReply{}, err
	}
	ip := address.To4()
	if ip == nil {
		return EchoReply{}, errors.New("gateway is not IPv4")
	}
	handle, _, _ := icmpCreateFile.Call()
	if handle == 0 || handle == invalidHandle {
		return EchoReply{}, ErrICMPUnavailable
	}
	defer icmpCloseHandle.Call(handle)
	payload := []byte("routecause")
	reply := make([]byte, 128)
	ms := timeout.Milliseconds()
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline).Milliseconds() < ms {
		ms = time.Until(deadline).Milliseconds()
	}
	if ms < 1 {
		return EchoReply{}, context.DeadlineExceeded
	}
	count, _, callErr := icmpSendEcho.Call(handle, uintptr(binary.LittleEndian.Uint32(ip)), uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), 0, uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), uintptr(ms))
	if count == 0 {
		if ctx.Err() != nil {
			return EchoReply{}, ctx.Err()
		}
		if callErr != syscall.Errno(0) {
			return EchoReply{}, fmt.Errorf("IcmpSendEcho: %w", callErr)
		}
		return EchoReply{}, context.DeadlineExceeded
	}
	status := binary.LittleEndian.Uint32(reply[4:8])
	if status != 0 {
		if status == 11010 {
			return EchoReply{}, context.DeadlineExceeded
		}
		return EchoReply{}, fmt.Errorf("IcmpSendEcho status %d", status)
	}
	rtt := binary.LittleEndian.Uint32(reply[8:12])
	return EchoReply{Latency: time.Duration(rtt) * time.Millisecond, Method: "windows_icmp"}, nil
}

func newNetworkAPIs() NetworkAPIs {
	return NetworkAPIs{WiFi: windowsWiFiAPI{}, Route: windowsRouteAPI{}, Echo: windowsEchoAPI{}, NativeWindows: true}
}
