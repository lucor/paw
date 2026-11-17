//go:build android

// Package netif enumerates network interfaces and their assigned IP addresses.
// On Android, it uses RTM_GETADDR and ioctl without binding a Netlink socket.
// Its approach is inspired by Android Bionic's getifaddrs implementation:
// https://android.googlesource.com/platform/bionic/
package netif

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"syscall"
	"unsafe"
)

const nlmFDumpIntr = 0x10 // NLM_F_DUMP_INTR: a dump was interrupted.
// Info pairs an interface with its assigned IP addresses from one snapshot.
type Info struct {
	// Interface contains the interface index, name, MTU, and flags.
	Interface net.Interface
	// Addrs contains the interface IP addresses with their network prefixes.
	Addrs []net.Addr
}

// Interfaces returns the network interfaces visible to the current process.
func Interfaces() ([]net.Interface, error) {
	snap, err := Snapshot()
	if err != nil {
		return nil, err
	}
	result := make([]net.Interface, 0, len(snap))
	for _, s := range snap {
		result = append(result, s.Interface)
	}
	return result, nil
}

// InterfaceAddrs returns the IP addresses assigned to visible interfaces.
func InterfaceAddrs() ([]net.Addr, error) {
	snap, err := Snapshot()
	if err != nil {
		return nil, err
	}
	var result []net.Addr
	for _, s := range snap {
		result = append(result, s.Addrs...)
	}
	return result, nil
}

// Addrs returns the IP addresses assigned to ifi, matched by interface index.
func Addrs(ifi net.Interface) ([]net.Addr, error) {
	snap, err := Snapshot()
	if err != nil {
		return nil, err
	}
	for _, s := range snap {
		if s.Interface.Index == ifi.Index {
			return append([]net.Addr(nil), s.Addrs...), nil
		}
	}
	return nil, fmt.Errorf("interface index %d not found", ifi.Index)
}

// Snapshot returns interfaces and their assigned IP addresses from one enumeration.
func Snapshot() ([]Info, error) {
	messages, err := getAddrMessages()
	if err != nil {
		return nil, err
	}
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open ioctl socket: %w", err)
	}
	defer syscall.Close(fd)

	byIndex := make(map[int]*Info)
	for _, m := range messages {
		if m.Header.Type != syscall.RTM_NEWADDR {
			continue
		}
		if len(m.Data) < syscall.SizeofIfAddrmsg {
			return nil, errors.New("short RTM_NEWADDR message")
		}
		index := int(binary.NativeEndian.Uint32(m.Data[4:8]))
		if index <= 0 {
			return nil, fmt.Errorf("invalid interface index %d", index)
		}
		attrs, err := syscall.ParseNetlinkRouteAttr(&m)
		if err != nil {
			return nil, fmt.Errorf("parse netlink attrs for %d: %w", index, err)
		}
		addr, err := parseIPAddr(m.Data[0], int(m.Data[1]), attrs)
		if err != nil {
			return nil, fmt.Errorf("parse address for %d: %w", index, err)
		}
		if addr == nil {
			continue
		}
		info, ok := byIndex[index]
		if !ok {
			ifi, err := interfaceFromIndex(fd, index)
			if err != nil {
				return nil, fmt.Errorf("interface %d: %w", index, err)
			}
			info = &Info{Interface: ifi}
			byIndex[index] = info
		}
		info.Addrs = append(info.Addrs, addr)
	}
	if len(byIndex) == 0 {
		return nil, errors.New("RTM_GETADDR returned no usable interfaces")
	}
	out := make([]Info, 0, len(byIndex))
	for _, info := range byIndex {
		out = append(out, *info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Interface.Index < out[j].Interface.Index })
	return out, nil
}

func parseIPAddr(family byte, prefix int, attrs []syscall.NetlinkRouteAttr) (*net.IPNet, error) {
	bits := 0
	switch int(family) {
	case syscall.AF_INET:
		bits = 32
	case syscall.AF_INET6:
		bits = 128
	default:
		return nil, nil
	}
	if prefix > bits {
		return nil, fmt.Errorf("invalid prefix %d for family %d", prefix, family)
	}
	var local, address []byte
	for _, a := range attrs {
		switch a.Attr.Type {
		case syscall.IFA_LOCAL:
			local = a.Value
		case syscall.IFA_ADDRESS:
			address = a.Value
		}
	}
	raw := address
	if len(local) > 0 {
		raw = local
	}
	if len(raw) != bits/8 {
		return nil, fmt.Errorf("invalid IP length: want %d got %d", bits/8, len(raw))
	}
	return &net.IPNet{IP: append(net.IP(nil), raw...), Mask: net.CIDRMask(prefix, bits)}, nil
}

func getAddrMessages() ([]syscall.NetlinkMessage, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW|syscall.SOCK_CLOEXEC, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil, fmt.Errorf("open NETLINK_ROUTE: %w", err)
	}
	defer syscall.Close(fd)
	// Intentionally no syscall.Bind: Android SELinux restricts netlink bind.
	const seq = 1
	request := make([]byte, 20) // nlmsghdr (16) + rtgenmsg (1) + padding (3)
	binary.NativeEndian.PutUint32(request[0:4], uint32(len(request)))
	binary.NativeEndian.PutUint16(request[4:6], syscall.RTM_GETADDR)
	binary.NativeEndian.PutUint16(request[6:8], syscall.NLM_F_REQUEST|syscall.NLM_F_DUMP)
	binary.NativeEndian.PutUint32(request[8:12], seq)
	request[16] = syscall.AF_UNSPEC
	if err := syscall.Sendto(fd, request, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return nil, fmt.Errorf("RTM_GETADDR send: %w", err)
	}
	sa, err := syscall.Getsockname(fd)
	if err != nil {
		return nil, err
	}
	local, ok := sa.(*syscall.SockaddrNetlink)
	if !ok {
		return nil, errors.New("unexpected netlink socket address")
	}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &syscall.Timeval{Sec: 3}); err != nil {
		return nil, err
	}
	var out []syscall.NetlinkMessage
	buf := make([]byte, 65536)
	for {
		n, _, flags, from, err := syscall.Recvmsg(fd, buf, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("RTM_GETADDR receive: %w", err)
		}
		if flags&syscall.MSG_TRUNC != 0 {
			return nil, errors.New("truncated netlink response")
		}
		nlfrom, ok := from.(*syscall.SockaddrNetlink)
		if !ok || nlfrom.Pid != 0 {
			return nil, errors.New("unexpected netlink sender")
		}
		msgs, err := syscall.ParseNetlinkMessage(buf[:n])
		if err != nil {
			return nil, err
		}
		if len(msgs) == 0 {
			return nil, errors.New("empty netlink response")
		}
		for _, m := range msgs {
			if m.Header.Seq != seq || m.Header.Pid != local.Pid {
				return nil, fmt.Errorf("unexpected response seq/pid: %d/%d want %d/%d", m.Header.Seq, m.Header.Pid, seq, local.Pid)
			}
			switch m.Header.Type {
			case syscall.NLMSG_DONE:
				if m.Header.Flags&nlmFDumpIntr != 0 {
					return nil, errors.New("netlink dump interrupted")
				}
				if len(m.Data) > 0 {
					if len(m.Data) < 4 {
						return nil, errors.New("short netlink done status")
					}
					status := int32(binary.NativeEndian.Uint32(m.Data[:4]))
					if status < 0 {
						return nil, fmt.Errorf("RTM_GETADDR done: %w", syscall.Errno(-status))
					}
					if status > 0 {
						return nil, fmt.Errorf("invalid netlink done status %d", status)
					}
				}
				return out, nil
			case syscall.NLMSG_ERROR:
				if len(m.Data) < 4 {
					return nil, errors.New("short netlink error")
				}
				status := int32(binary.NativeEndian.Uint32(m.Data[:4]))
				if status < 0 {
					return nil, fmt.Errorf("RTM_GETADDR: %w", syscall.Errno(-status))
				}
				if status > 0 {
					return nil, fmt.Errorf("invalid netlink error status %d", status)
				}
			case syscall.RTM_NEWADDR:
				m.Data = append([]byte(nil), m.Data...)
				out = append(out, m)
			}
		}
	}
}

// 40 bytes is >= sizeof(struct ifreq) on Android 32- and 64-bit ABIs.
// The kernel ioctl ABI reads/writes the first 16 bytes for the name and
// the union starting at byte 16. The uint64 field ensures alignment.
type ifreq struct {
	Name [syscall.IFNAMSIZ]byte
	Data [24]byte
	_    [0]uint64
}

func ioctl(fd int, request uint, ifr *ifreq) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(request), uintptr(unsafe.Pointer(ifr)))
	if errno != 0 {
		return errno
	}
	return nil
}

func interfaceFromIndex(fd, index int) (net.Interface, error) {
	var req ifreq
	binary.NativeEndian.PutUint32(req.Data[0:4], uint32(index))
	if err := ioctl(fd, syscall.SIOCGIFNAME, &req); err != nil {
		return net.Interface{}, fmt.Errorf("SIOCGIFNAME: %w", err)
	}
	k := bytes.IndexByte(req.Name[:], 0)
	if k <= 0 {
		return net.Interface{}, errors.New("invalid interface name")
	}
	name := string(req.Name[:k])
	var mtuReq ifreq
	copy(mtuReq.Name[:], name)
	if err := ioctl(fd, syscall.SIOCGIFMTU, &mtuReq); err != nil {
		return net.Interface{}, fmt.Errorf("SIOCGIFMTU %q: %w", name, err)
	}
	var flagReq ifreq
	copy(flagReq.Name[:], name)
	if err := ioctl(fd, syscall.SIOCGIFFLAGS, &flagReq); err != nil {
		return net.Interface{}, fmt.Errorf("SIOCGIFFLAGS %q: %w", name, err)
	}
	raw := int(binary.NativeEndian.Uint16(flagReq.Data[0:2]))
	var flags net.Flags
	if raw&syscall.IFF_UP != 0 {
		flags |= net.FlagUp
	}
	if raw&syscall.IFF_BROADCAST != 0 {
		flags |= net.FlagBroadcast
	}
	if raw&syscall.IFF_LOOPBACK != 0 {
		flags |= net.FlagLoopback
	}
	if raw&syscall.IFF_POINTOPOINT != 0 {
		flags |= net.FlagPointToPoint
	}
	if raw&syscall.IFF_MULTICAST != 0 {
		flags |= net.FlagMulticast
	}
	if raw&syscall.IFF_RUNNING != 0 {
		flags |= net.FlagRunning
	}
	return net.Interface{
		Index: index,
		Name:  name,
		MTU:   int(int32(binary.NativeEndian.Uint32(mtuReq.Data[:4]))),
		Flags: flags,
	}, nil
}
