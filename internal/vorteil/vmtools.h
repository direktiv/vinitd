#ifndef _VMTOOLS_H
#define _VMTOOLS_H

// #include <stdbool.h>
#include <stdint.h>

int vmtools_start(int cards, char *hostname);

#define VMT_RPC_BUFLEN 4096

#define VM_MAGIC 0x564D5868

#define VM_RPC_FLAG_COOKIE 0x80000000UL

// commands
#define VM_CMD_RPC 0x1e

// sub-commands,
#define VM_RPC_OPEN 0x00
#define VM_RPC_SET_LENGTH 0x01
#define VM_RPC_GET_LENGTH		0x03
#define VM_RPC_GET_END			0x05
#define VM_RPC_CLOSE			0x06

// magic numbers
#define VM_RPC_OPEN_RPCI 0x49435052UL
#define VM_RPC_OPEN_TCLO 0x4F4C4354UL
#define VM_RPC_ENH_DATA 0x00010000UL

// port numbers
#define VM_PORT_CMD 0x5658
#define VM_PORT_RPC 0x5659

// reply flags
#define VM_RPC_REPLY_SUCCESS 0x0001
#define VM_RPC_REPLY_DORECV	0x0002

// rpc response
#define VM_RPC_REPLY_OK "OK "
#define VM_RPC_RESET_REPLY		"OK ATR toolbox"
#define VM_RPC_REPLY_ERROR		"ERROR Unknown command"
// #define VM_RPC_REPLY_ERROR_IP_ADDR	"ERROR Unable to find guest IP address"

// vm state
#define VM_STATE_CHANGE_HALT	1
#define VM_STATE_CHANGE_REBOOT	2
#define VM_STATE_CHANGE_POWERON 3
#define VM_STATE_CHANGE_RESUME  4
#define VM_STATE_CHANGE_SUSPEND 5

// guest info keys
#define VM_GUEST_INFO_DNS_NAME		1
#define VM_GUEST_INFO_IP_ADDRESS	2
#define VM_GUEST_INFO_DISK_FREE_SPACE	3
#define VM_GUEST_INFO_BUILD_NUMBER	4
#define VM_GUEST_INFO_OS_NAME_FULL	5
#define VM_GUEST_INFO_OS_NAME		6
#define VM_GUEST_INFO_UPTIME		7
#define VM_GUEST_INFO_MEMORY		8
#define VM_GUEST_INFO_IP_ADDRESS_V2	9


#define  VM_VERSION_UNMANAGED			0x7fffffff

#define UNUSED_RET(x) (void)((x) + 1)

struct vm_rpc {
	uint16_t channel;
	uint32_t cookie1;
	uint32_t cookie2;

	int sc_rpc_error;
	int sc_tclo_ping;
 	int sc_set_guest_os;

	char *sc_rpc_buf;
};

/* A register. */
union vm_reg {
	struct {
		uint16_t low;
		uint16_t high;
	} part;
	uint32_t word;
	struct {
		uint32_t low;
		uint32_t high;
	} words;
	uint64_t quad;
} __attribute__((packed));

struct vm_backdoor {
	union vm_reg eax;
	union vm_reg ebx;
	union vm_reg ecx;
	union vm_reg edx;
	union vm_reg esi;
	union vm_reg edi;
	union vm_reg ebp;
} __attribute__((packed));

#define BACKDOOR_OP(op, frame)                                                                                                                                 \
	__asm__ volatile("pushq %%rbp;			\n\t"                                                                                                                 \
			 "pushq %%rax;			\n\t"                                                                                                                 \
			 "movq 0x30(%%rax), %%rbp;	\n\t"                                                                                                       \
			 "movq 0x28(%%rax), %%rdi;	\n\t"                                                                                                       \
			 "movq 0x20(%%rax), %%rsi;	\n\t"                                                                                                       \
			 "movq 0x18(%%rax), %%rdx;	\n\t"                                                                                                       \
			 "movq 0x10(%%rax), %%rcx;	\n\t"                                                                                                       \
			 "movq 0x08(%%rax), %%rbx;	\n\t"                                                                                                       \
			 "movq 0x00(%%rax), %%rax;	\n\t" op "\n\t"                                                                                       \
			 "xchgq %%rax, 0x00(%%rsp);	\n\t"                                                                                                      \
			 "movq %%rbp, 0x30(%%rax);	\n\t"                                                                                                       \
			 "movq %%rdi, 0x28(%%rax);	\n\t"                                                                                                       \
			 "movq %%rsi, 0x20(%%rax);	\n\t"                                                                                                       \
			 "movq %%rdx, 0x18(%%rax);	\n\t"                                                                                                       \
			 "movq %%rcx, 0x10(%%rax);	\n\t"                                                                                                       \
			 "movq %%rbx, 0x08(%%rax);	\n\t"                                                                                                       \
			 "popq 0x00(%%rax);		\n\t"                                                                                                             \
			 "popq %%rbp;			\n\t"                                                                                                                  \
			 : /* No outputs. */                                                                                                                   \
			 : "a"(frame) /* No pushal on amd64 so warn gcc about the clobbered registers. */                                                      \
			 : "rbx", "rcx", "rdx", "rdi", "rsi", "cc", "memory")



#endif
