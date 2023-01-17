package uring

import (
	"fmt"
	"syscall"
	"os"
	"unsafe"
)

// read from
// https://github.com/axboe/liburing/blob/0df8a379e929641699c2ab1f42de1efd2515b908/src/include/liburing.h
const (
	sysSetup uintptr = 425
	sysEnter uintptr = 426
	sysRegister uintptr = 427
)

// https://github.com/axboe/liburing/blob/48c26c1fa3b76b9bb618ce12f639fbbdad99b82c/src/include/liburing/io_uring.h#L367-L372
const (
	sqRingOff uint64 = 0
	cqRingOff uint64 = 0x8000000
	sqesOff uint64 = 0x10000000
)

// https://github.com/axboe/liburing/blob/48c26c1fa3b76b9bb618ce12f639fbbdad99b82c/src/include/liburing/io_uring.h#L440-L455

const (
	featSingleMMap uint32 = 1 << 0
	featNoDrop uint32 = 1 << 1
	featSubmitStable uint32 = 1 << 2
	featRWCurPos uint32 = 1 << 3
	featCurPersonality uint32 = 1 << 4
	featFastPool uint32 = 1 << 5
	featPoll32Bits uint32 = 1 << 6
	featSQPollNonfixed uint32 = 1 << 7
	featExtArg uint32 = 1 << 8
	featNativeWorkers uint32 = 1 << 9
	featRSrcTags uint32 = 1 << 10
	featCQESkip uint32 = 1 << 11
	featFeatLinkedFile uint32 = 1 << 12
)

type uring struct {
	fd int
	params *uringParams
	sqRing *sqRing
	cqRing *cqRing
	sqeBuffer []byte
}

type sqRing struct {
	size uint64
	buffer []byte
	head *uint32
	tail *uint32
	ringMask *uint32
	ringEntries *uint32
	flags *uint32
	dropped *uint32
	array *uint32
}

type cqRing struct {
	size uint64
	buffer []byte
	head *uint32
	tail *uint32
	ringMask *uint32
	ringEntries *uint32
	overflow *uint32
	flags *uint32
	currentCQE *CQE
}

type CQE struct{
	userData uint64
	res int32
	flags uint32
}

type SQE struct {
	opcode uint8
	flags uint8
	ioprio uint16
	fd int32
	off uint64
	addr uint64
	len uint32
	opcodeFlags uint32
	userData uint64
	bufIndex uint16
	personality uint16
	spliceFdIn uint32
	_pad2 [2]uint64
}

type uringParams struct {
	sqEntries uint32
	cqEntries uint32
	flags uint32
	sqThreadCpu uint32
	sqThreadIdle uint32
	features uint32
	wqFd uint32
	resv [5]uint32
	sqRingOffsets sqRingParams
	cqRingOffsets cqRingParams
}

type sqRingParams struct {
	head uint32
	tail uint32
	ringMask uint32
	ringEntries uint32
	flags uint32
	dropped uint32
	array uint32
	rsv1 uint32
	rsv2 uint64
}

type cqRingParams struct {
	head uint32
	tail uint32
	ringMask uint32
	ringEntries uint32
	overflow uint32
	cqes uint32
	flags uint32
	rsv1 uint32
	rsv2 uint64
}

func (p *uringParams) singleMMap() bool {
	return p.features & featSingleMMap != 0
}
	

func NewUring(entries uint32) *uring {
	params := &uringParams{}
	fd, err := setupRing(entries, params)

	if err != nil {
		panic(err)
	}

	u := &uring{
		fd: fd,
		params: params,
		sqRing: &sqRing{},
		cqRing: &cqRing{},
	}

	u.setSizes()
	if err = u.mmapRing(); err != nil {
		panic(err)
	}

	return u
}

func setupRing(entries uint32, params *uringParams) (int, error) {
	fd, _, errno := syscall.Syscall(sysSetup, uintptr(entries), uintptr(unsafe.Pointer(params)), 0)

	if errno != 0 {
		return int(fd), os.NewSyscallError("io_uring_setup", errno)
	}

	return int(fd), nil
}

func (u *uring) setSizes() {
	u.sqRing.size = uint64(u.params.sqRingOffsets.array) + uint64(u.params.sqRingOffsets.ringEntries) * uint64(unsafe.Sizeof(uint32(0)))
	u.cqRing.size = uint64(u.params.cqRingOffsets.cqes) + uint64(u.params.cqRingOffsets.ringEntries) * uint64(unsafe.Sizeof(CQE{}))
}


func (u *uring) mmapRing() error {
	data, err := syscall.Mmap(
		u.fd,
		int64(sqRingOff),
		int(u.sqRing.size),
		syscall.PROT_READ | syscall.PROT_WRITE,
		syscall.MAP_SHARED | syscall.MAP_POPULATE,
	)

	if err != nil {
		return nil
	}

	u.sqRing.buffer = data

	if u.params.singleMMap() {
		if u.cqRing.size > u.sqRing.size {
			u.sqRing.size = u.cqRing.size
		}
		u.cqRing.size = u.sqRing.size
	}
			
	if u.params.singleMMap() {
		u.cqRing.buffer = u.sqRing.buffer
	} else {
		data, err = syscall.Mmap(
			u.fd,
			int64(cqRingOff),
			int(u.cqRing.size),
			syscall.PROT_READ | syscall.PROT_WRITE,
			syscall.MAP_SHARED | syscall.MAP_POPULATE,
		)

		if err != nil {
			return err
		}

		u.cqRing.buffer = data
	}


	sqPtr := &u.sqRing.buffer[0]
	u.sqRing.head = getPointer(sqPtr, u.params.sqRingOffsets.head)
	u.sqRing.tail = getPointer(sqPtr, u.params.sqRingOffsets.tail)
	u.sqRing.ringMask = getPointer(sqPtr, u.params.sqRingOffsets.ringMask)
	u.sqRing.ringEntries = getPointer(sqPtr, u.params.sqRingOffsets.ringEntries)
	u.sqRing.flags = getPointer(sqPtr, u.params.sqRingOffsets.flags)
	u.sqRing.dropped = getPointer(sqPtr, u.params.sqRingOffsets.dropped)
	u.sqRing.array = getPointer(sqPtr, u.params.sqRingOffsets.array)

	sqeBufferSize := uintptr(u.params.sqEntries) * unsafe.Sizeof(SQE{})
	u.sqeBuffer, err = syscall.Mmap(
		u.fd,
		int64(sqesOff),
		int(sqeBufferSize),
		syscall.PROT_READ | syscall.PROT_WRITE,
		syscall.MAP_SHARED | syscall.MAP_POPULATE,
	)
		
	cqPtr := &u.cqRing.buffer[0]
	u.cqRing.head = getPointer(cqPtr, u.params.cqRingOffsets.head)
	u.cqRing.tail = getPointer(cqPtr, u.params.cqRingOffsets.tail)
	u.cqRing.ringMask = getPointer(cqPtr, u.params.cqRingOffsets.ringMask)
	u.cqRing.ringEntries = getPointer(cqPtr, u.params.cqRingOffsets.ringEntries)
	if u.params.cqRingOffsets.flags != 0 {
		u.cqRing.flags = getPointer(cqPtr, u.params.cqRingOffsets.flags)
	}

	return nil
}

func getPointer(b *byte, offset uint32) *uint32 {
	return (*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(b)) + uintptr(offset)))
}
	
	
func (u *uring) Inspect() string {
	return fmt.Sprintf("fd: %v, params: %+v", u.fd, u.params)
}
