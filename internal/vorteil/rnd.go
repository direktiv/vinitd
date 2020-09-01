package vorteil

import (
	"crypto/rand"
	"encoding/binary"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type randIn struct {
	count int
	size  int
	buf   uint64
}

// this function has to run successfully for e.g. DHCP
// if it fails we can just panic
func runRandom() error {

	var event unix.EpollEvent
	var events [1]unix.EpollEvent

	rndSrc, err := os.Open("/dev/random")
	if err != nil {
		SystemPanic("open /dev/random failed: %s", err.Error())
	}
	defer rndSrc.Close()
	fd := int(rndSrc.Fd())

	epfd, err := unix.EpollCreate1(0)
	if err != nil {
		SystemPanic("epoll error: %v", err)
	}
	defer unix.Close(epfd)

	event.Events = unix.EPOLLOUT
	event.Fd = int32(fd)
	if err := unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, fd, &event); err != nil {
		SystemPanic("epoll error: %v", err)
	}

	runEntropy(epfd, events, rndSrc)

	return nil
}

func runEntropy(epfd int, events [1]unix.EpollEvent, rnd *os.File) {

	for {
		n, err := unix.EpollWait(epfd, events[:], -1)
		if err != nil {
			if e, ok := err.(syscall.Errno); ok {
				if e.Temporary() {
					continue
				}
			}
		}

		// we have on event so we have to add entropy
		if n == 1 && events[0].Events&unix.EPOLLOUT == unix.EPOLLOUT {
			addEntropy(rnd)
		}
	}

}

func addEntropy(random *os.File) (int, error) {

	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return 0, nil
	}
	r := binary.LittleEndian.Uint64(b)

	const entropy = 64
	info := randIn{entropy, 8, r}
	ret, _, err := unix.Syscall(unix.SYS_IOCTL, uintptr(random.Fd()), uintptr(unix.RNDADDENTROPY), uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 8, nil
	}

	return 0, err
}
