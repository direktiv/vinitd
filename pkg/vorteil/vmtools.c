/*
 * Copyright (c) 2007 David Crawshaw <david@zentus.com>
 * Copyright (c) 2008 David Gwynne <dlg@openbsd.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 *
 * openbsd vmt.c, modified for vorteil
 * Copyright (c) 2020 Jens Gerke <jens.gerke@vorteil.io>
 *
 */
#include <stdlib.h>
#include <stddef.h>
#include <pthread.h>
#include <stdio.h>
#include <unistd.h>
#include <string.h>
#include <stdarg.h>
#include <ifaddrs.h>
#include <fcntl.h>

#include <net/if.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <sys/ioctl.h>
#include <netinet/in.h>
#include <arpa/inet.h>

#include "vmtools.h"

#define LOOP_WAIT_MS 5000

static int fdelay = 0;

static int numberOfCards = 0;
static char hostname[1024];

struct vm_rpc *rpc;

static int vm_rpc_open(struct vm_rpc *rpc, uint32_t proto);
static void vm_cmd(struct vm_backdoor *frame);
static void *run_req(void *arg);
static int vm_rpc_send_str(const struct vm_rpc *rpc, const uint8_t *str);
static int vm_rpc_send(const struct vm_rpc *rpc, const uint8_t *buf, uint32_t length);
static void vm_outs(struct vm_backdoor *frame);
static int vm_rpc_get_length(const struct vm_rpc *rpc, uint32_t *length, uint16_t *dataid);
static int vm_rpc_get_data(const struct vm_rpc *rpc, char *data, uint32_t length, uint16_t dataid);
static int vm_rpc_send_str(const struct vm_rpc *rpc, const uint8_t *str);
static void vm_ins(struct vm_backdoor *frame);
static int vmt_tclo_process(const char *name);
static void vmt_tclo_reset(void);
static int vm_rpc_close(struct vm_rpc *rpc);
static void vmt_tclo_capreg(void);
static int vm_rpc_send_rpci_tx_buf(struct vm_rpc *rpc, const uint8_t *buf, uint32_t length);
static int vm_rpc_send_rpci_tx(const char *fmt, ...);
static int vm_rpci_response_successful(void);
static void vmt_tclo_poweron(void);
static void vmt_tclo_ping(void);
static void vmt_update_guest_info(void);
static void vmt_tclo_broadcastip(void);
static int network_ioctl(unsigned long req, void *data);
static void set_network_info(int card_no, char *msg);
static void vmt_update_guest_uptime(void);
static void vmt_tclo_halt(void);
static void vmt_tclo_reboot(void);
static void vmt_tclo_resume(void);
static void vmt_tclo_suspend(void);

extern int UptimeForTools(void);
extern void ShutdownForTools(void);
extern void RebootForTools(void);

extern void err_print(char *txt);

char card1[120] = { 0x53, 0x65, 0x74, 0x47, 0x75, 0x65, 0x73, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x20, 0x20, 0x39, 0x20, 0x00, 0x00, 0x00, 0x03,
		    0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x11, 0x30, 0x30, 0x3a, 0x35, 0x30, 0x3a, 0x35, 0x36,
		    0x3a, 0x61, 0x31, 0x3a, 0x62, 0x65, 0x3a, 0x66, 0x38, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		    0x00, 0x00, 0x00, 0x04, 0x0a, 0x00, 0x00, 0x4f, 0x00, 0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		    0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00 };
int card1_len = 120;

char card2[192] = { 0x53, 0x65, 0x74, 0x47, 0x75, 0x65, 0x73, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x20, 0x20, 0x39, 0x20, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00,
		    0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x11, 0x30, 0x30, 0x3a, 0x35, 0x30, 0x3a, 0x35, 0x36, 0x3a, 0x61, 0x31, 0x3a,
		    0x62, 0x65, 0x3a, 0x66, 0x38, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x04, 0x0a, 0x00,
		    0x00, 0x4f, 0x00, 0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00,
		    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x11, 0x30, 0x30, 0x3a, 0x35, 0x30, 0x3a,
		    0x35, 0x36, 0x3a, 0x61, 0x31, 0x3a, 0x66, 0x30, 0x3a, 0x61, 0x61, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		    0x00, 0x00, 0x00, 0x04, 0x0a, 0x00, 0x00, 0x59, 0x00, 0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00,
		    0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00 };
int card2_len = 192;

char card3[264] = { 0x53, 0x65, 0x74, 0x47, 0x75, 0x65, 0x73, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x20, 0x20, 0x39, 0x20, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00,
		    0x00, 0x01, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x11, 0x30, 0x30, 0x3a, 0x35, 0x30, 0x3a, 0x35, 0x36, 0x3a, 0x61, 0x31, 0x3a,
		    0x62, 0x65, 0x3a, 0x66, 0x38, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x04, 0x0a, 0x00,
		    0x00, 0x4f, 0x00, 0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00,
		    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x11, 0x30, 0x30, 0x3a, 0x35, 0x30, 0x3a,
		    0x35, 0x36, 0x3a, 0x61, 0x31, 0x3a, 0x66, 0x30, 0x3a, 0x61, 0x61, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		    0x00, 0x00, 0x00, 0x04, 0x0a, 0x00, 0x00, 0x59, 0x00, 0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00,
		    0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x11,
		    0x30, 0x30, 0x3a, 0x35, 0x30, 0x3a, 0x35, 0x36, 0x3a, 0x61, 0x31, 0x3a, 0x65, 0x65, 0x3a, 0x39, 0x63, 0x00, 0x00, 0x00, 0x00, 0x00,
		    0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x04, 0x0a, 0x00, 0x00, 0x5a, 0x00, 0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00,
		    0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00 };
int card3_len = 264;

char card4[336] = { 0x53, 0x65, 0x74, 0x47, 0x75, 0x65, 0x73, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x20, 0x20, 0x39, 0x20, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00,
		    0x01, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x11, 0x30, 0x30, 0x3a, 0x35, 0x30, 0x3a, 0x35, 0x36, 0x3a, 0x61, 0x31, 0x3a, 0x62, 0x65,
		    0x3a, 0x66, 0x38, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x04, 0x0a, 0x00, 0x00, 0x4f, 0x00,
		    0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x11, 0x30, 0x30, 0x3a, 0x35, 0x30, 0x3a, 0x35, 0x36, 0x3a, 0x61, 0x31,
		    0x3a, 0x66, 0x30, 0x3a, 0x61, 0x61, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x04, 0x0a, 0x00,
		    0x00, 0x59, 0x00, 0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00,
		    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x11, 0x30, 0x30, 0x3a, 0x35, 0x30, 0x3a, 0x35, 0x36,
		    0x3a, 0x61, 0x31, 0x3a, 0x65, 0x65, 0x3a, 0x39, 0x63, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		    0x04, 0x0a, 0x00, 0x00, 0x5a, 0x00, 0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00,
		    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x11, 0x30, 0x30, 0x3a, 0x35, 0x30,
		    0x3a, 0x35, 0x36, 0x3a, 0x61, 0x31, 0x3a, 0x31, 0x64, 0x3a, 0x30, 0x38, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		    0x00, 0x00, 0x00, 0x04, 0x0a, 0x00, 0x00, 0x5b, 0x00, 0x00, 0x00, 0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		    0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00 };
int card4_len = 336;

struct vmt_tclo_rpc {
	const char *name;
	void (*cb)();
} vmt_tclo_rpc[] = {
	/* Keep sorted by name (case-sensitive) */
	{ "Capabilities_Register", vmt_tclo_capreg },
	{ "OS_Halt", vmt_tclo_halt },
	{ "OS_PowerOn", vmt_tclo_poweron },
	{ "OS_Reboot", vmt_tclo_reboot },
	{ "Set_Option broadcastIP 1", vmt_tclo_broadcastip },
	{ "ping", vmt_tclo_ping },
	{ "reset", vmt_tclo_reset },
	{ "OS_Resume", vmt_tclo_resume },
	{ "OS_Suspend", vmt_tclo_suspend },
	{ NULL },
};

static void *run_req(void *arg)
{
	u_int32_t rlen = 0;
	u_int16_t ack;
	int delay = 0;

	while (1) {
		delay = fdelay;

		// that prevents 100% CPU loops if for some reason the tools
		// are not responding right
		if (fdelay < LOOP_WAIT_MS) {
			fdelay += 5;
		}

		if (rpc->channel == 0 && rpc->cookie1 == 0 && rpc->cookie2 == 0) {

			fdelay = 0;
			delay = 0;

			if (vm_rpc_open(rpc, VM_RPC_OPEN_TCLO) != 0) {
				err_print("unable to reopen TCLO channel");
				fdelay = 15;
				goto out;
			}

			if (vm_rpc_send_str(rpc, (const uint8_t *)VM_RPC_RESET_REPLY) != 0) {
				err_print("failed to send reset reply");
				rpc->sc_rpc_error = 1;
				goto out;
			} else {
				rpc->sc_rpc_error = 0;
			}
		}

		if (rpc->sc_tclo_ping) {
			if (vm_rpc_send(rpc, NULL, 0) != 0) {
				err_print("failed to send TCLO outgoing ping");
				rpc->sc_rpc_error = 1;
				goto out;
			}
		}

		rlen = 0;
		memset(rpc->sc_rpc_buf, 0, VMT_RPC_BUFLEN);

		if (vm_rpc_get_length(rpc, &rlen, &ack) != 0) {
			err_print("failed to get length of incoming TCLO data");
			rpc->sc_rpc_error = 1;
			goto out;
		}

		if (rlen == 0) {
			rpc->sc_tclo_ping = 1;
			goto out;
		}

		if (rlen >= VMT_RPC_BUFLEN) {
			rlen = VMT_RPC_BUFLEN - 1;
		}


		if (vm_rpc_get_data(rpc, rpc->sc_rpc_buf, rlen, ack) != 0) {
			err_print("failed to get incoming TCLO data");
			rpc->sc_rpc_error = 1;
			goto out;
		}

		rpc->sc_tclo_ping = 0;

		if (vmt_tclo_process(rpc->sc_rpc_buf) != 0) {
			if (vm_rpc_send_str(rpc, (const uint8_t *)VM_RPC_REPLY_ERROR) != 0) {
				err_print("error sending unknown command reply");
				rpc->sc_rpc_error = 1;
			}
		}

	out:
		if (rpc->sc_rpc_error == 1) {
			if (vm_rpc_close(rpc) == 0) {
				rpc->sc_rpc_error = 0;
			}
		}

		usleep(delay * 1000);
	}

	return NULL;
}

static int vmt_tclo_process(const char *name)
{
	int i;

	/* Search for rpc command and call handler */
	for (i = 0; vmt_tclo_rpc[i].name != NULL; i++) {
		if (strcmp(vmt_tclo_rpc[i].name, rpc->sc_rpc_buf) == 0) {
			vmt_tclo_rpc[i].cb();
			return (0);
		}
	}
	return (-1);
}

int vmtools_start(int cards, char *hn)
{
	numberOfCards = cards;
	pthread_t thread_id;

	strcpy(hostname, hn);

	rpc = calloc(1, sizeof(struct vm_rpc));
	rpc->sc_rpc_buf = calloc(1, VMT_RPC_BUFLEN);

	pthread_create(&thread_id, NULL, run_req, NULL);

	return 0;
}

static void vm_ins(struct vm_backdoor *frame)
{
	BACKDOOR_OP("cld;\n\trep insb;", frame);
}

static void vm_outs(struct vm_backdoor *frame)
{
	BACKDOOR_OP("cld;\n\trep outsb;", frame);
}

static void vm_cmd(struct vm_backdoor *frame)
{
	BACKDOOR_OP("inl %%dx, %%eax;", frame);
}

static int vm_rpc_open(struct vm_rpc *rpc, uint32_t proto)
{
	struct vm_backdoor frame;

	bzero(&frame, sizeof(frame));
	frame.eax.word = VM_MAGIC;
	frame.ebx.word = proto | VM_RPC_FLAG_COOKIE;
	frame.ecx.part.low = VM_CMD_RPC;
	frame.ecx.part.high = VM_RPC_OPEN;
	frame.edx.part.low = VM_PORT_CMD;
	frame.edx.part.high = 0;

	vm_cmd(&frame);

	if (frame.ecx.part.high != 1 || frame.edx.part.low != 0) {
		/* open-vm-tools retries without VM_RPC_FLAG_COOKIE here.. */
		err_print("vmware: open failed");
		return 1;
	}

	rpc->channel = frame.edx.part.high;
	rpc->cookie1 = frame.esi.word;
	rpc->cookie2 = frame.edi.word;

	return 0;
}

static int vm_rpc_send_str(const struct vm_rpc *rpc, const uint8_t *str)
{
	return vm_rpc_send(rpc, str, strlen((char *)str));
}

static int vm_rpc_send(const struct vm_rpc *rpc, const uint8_t *buf, uint32_t length)
{
	struct vm_backdoor frame;

	bzero(&frame, sizeof(frame));
	frame.eax.word = VM_MAGIC;
	frame.ebx.word = length;
	frame.ecx.part.low = VM_CMD_RPC;
	frame.ecx.part.high = VM_RPC_SET_LENGTH;
	frame.edx.part.low = VM_PORT_CMD;
	frame.edx.part.high = rpc->channel;
	frame.esi.word = rpc->cookie1;
	frame.edi.word = rpc->cookie2;

	vm_cmd(&frame);

	if ((frame.ecx.part.high & VM_RPC_REPLY_SUCCESS) == 0) {
		err_print("vmware: sending length failed");
		return 1;
	}

	if (length == 0)
		return 0;

	/* Send the command using enhanced RPC. */
	bzero(&frame, sizeof(frame));
	frame.eax.word = VM_MAGIC;
	frame.ebx.word = VM_RPC_ENH_DATA;
	frame.ecx.word = length;
	frame.edx.part.low = VM_PORT_RPC;
	frame.edx.part.high = rpc->channel;
	frame.ebp.word = rpc->cookie1;
	frame.edi.word = rpc->cookie2;
	frame.esi.quad = (uint64_t)buf;

	vm_outs(&frame);

	if (frame.ebx.word != VM_RPC_ENH_DATA) {
		err_print("vmtools send failed");
		return 1;
	}

	return 0;
}

static int vm_rpc_get_length(const struct vm_rpc *rpc, uint32_t *length, uint16_t *dataid)
{
	struct vm_backdoor frame;

	bzero(&frame, sizeof(frame));
	frame.eax.word = VM_MAGIC;
	frame.ebx.word = 0;
	frame.ecx.part.low = VM_CMD_RPC;
	frame.ecx.part.high = VM_RPC_GET_LENGTH;
	frame.edx.part.low = VM_PORT_CMD;
	frame.edx.part.high = rpc->channel;
	frame.esi.word = rpc->cookie1;
	frame.edi.word = rpc->cookie2;

	vm_cmd(&frame);

	if ((frame.ecx.part.high & VM_RPC_REPLY_SUCCESS) == 0) {
		err_print("vmware get length failed");
		return 1;
	}
	if ((frame.ecx.part.high & VM_RPC_REPLY_DORECV) == 0) {
		*length = 0;
		*dataid = 0;
	} else {
		*length = frame.ebx.word;
		*dataid = frame.edx.part.high;
	}

	return 0;
}

static int vm_rpc_get_data(const struct vm_rpc *rpc, char *data, uint32_t length, uint16_t dataid)
{
	struct vm_backdoor frame;

	/* Get data using enhanced RPC. */
	bzero(&frame, sizeof(frame));
	frame.eax.word = VM_MAGIC;
	frame.ebx.word = VM_RPC_ENH_DATA;
	frame.ecx.word = length;
	frame.edx.part.low = VM_PORT_RPC;
	frame.edx.part.high = rpc->channel;
	frame.esi.word = rpc->cookie1;
	frame.edi.quad = (uint64_t)data;
	frame.ebp.word = rpc->cookie2;

	vm_ins(&frame);

	/* NUL-terminate the data */
	data[length] = '\0';

	if (frame.ebx.word != VM_RPC_ENH_DATA) {
		err_print("vmware get data failed");
		return 1;
	}

	/* Acknowledge data received. */
	bzero(&frame, sizeof(frame));
	frame.eax.word = VM_MAGIC;
	frame.ebx.word = dataid;
	frame.ecx.part.low = VM_CMD_RPC;
	frame.ecx.part.high = VM_RPC_GET_END;
	frame.edx.part.low = VM_PORT_CMD;
	frame.edx.part.high = rpc->channel;
	frame.esi.word = rpc->cookie1;
	frame.edi.word = rpc->cookie2;

	vm_cmd(&frame);

	if (frame.ecx.part.high == 0) {
		err_print("vmware ack data failed");
		return 1;
	}

	return 0;
}

static int vm_rpc_close(struct vm_rpc *rpc)
{
	struct vm_backdoor frame;

	bzero(&frame, sizeof(frame));
	frame.eax.word = VM_MAGIC;
	frame.ebx.word = 0;
	frame.ecx.part.low = VM_CMD_RPC;
	frame.ecx.part.high = VM_RPC_CLOSE;
	frame.edx.part.low = VM_PORT_CMD;
	frame.edx.part.high = rpc->channel;
	frame.edi.word = rpc->cookie2;
	frame.esi.word = rpc->cookie1;

	vm_cmd(&frame);

	if (frame.ecx.part.high == 0 || frame.ecx.part.low != 0) {
		err_print("vmware close failed");
	}

	rpc->channel = 0;
	rpc->cookie1 = 0;
	rpc->cookie2 = 0;
	rpc->sc_tclo_ping = 0;

	return 0;
}

static int vm_rpc_send_rpci_tx_buf(struct vm_rpc *rpc, const uint8_t *buf, uint32_t length)
{
	struct vm_rpc rpci;
	u_int32_t rlen;
	u_int16_t ack;
	int result = 0;

	if (vm_rpc_open(&rpci, VM_RPC_OPEN_RPCI) != 0) {
		err_print("rpci channel open failed");
		return 1;
	}

	if (vm_rpc_send(&rpci, (const uint8_t *)rpc->sc_rpc_buf, length) != 0) {
		err_print("unable to send rpci command");
		result = 1;
		goto out;
	}

	if (vm_rpc_get_length(&rpci, &rlen, &ack) != 0) {
		err_print("failed to get length of rpci response data");
		result = 1;
		goto out;
	}

	if (rlen > 0) {
		if (rlen >= VMT_RPC_BUFLEN) {
			rlen = VMT_RPC_BUFLEN - 1;
		}

		if (vm_rpc_get_data(&rpci, rpc->sc_rpc_buf, rlen, ack) != 0) {
			err_print("failed to get rpci response data");
			result = 1;
			goto out;
		}
	}

out:
	if (vm_rpc_close(&rpci) != 0) {
		err_print("unable to close rpci channel");
	}

	return result;
}

static void vmt_tclo_reset(void)
{
	if (rpc->sc_rpc_error != 0) {
		err_print("resetting rpc");
		vm_rpc_close(rpc);

		/* reopen and send the reset reply next time around */
		rpc->sc_rpc_error = 1;
		return;
	}

	if (vm_rpc_send_str(rpc, (const uint8_t *)VM_RPC_RESET_REPLY) != 0) {
		err_print("failed to send reset reply");
		rpc->sc_rpc_error = 1;
	}
}

static int vm_rpc_send_rpci_tx(const char *fmt, ...)
{
	va_list args;
	int len;

	va_start(args, fmt);
	len = vsnprintf(rpc->sc_rpc_buf, VMT_RPC_BUFLEN, fmt, args);
	va_end(args);

	if (len >= VMT_RPC_BUFLEN) {
		err_print("%s: rpci command didn't fit in buffer");
		return 1;
	}

	return vm_rpc_send_rpci_tx_buf(rpc, (const uint8_t *)rpc->sc_rpc_buf, len);
}

static int vm_rpci_response_successful(void)
{
	return (rpc->sc_rpc_buf[0] == '1' && rpc->sc_rpc_buf[1] == ' ');
}

static void vmt_tclo_capreg(void)
{
	/* don't know if this is important at all */
	if (vm_rpc_send_rpci_tx("vmx.capability.unified_loop toolbox") != 0) {
		err_print("unable to set unified loop");
		rpc->sc_rpc_error = 1;
	}

	if (vm_rpci_response_successful() == 0) {
		err_print("host rejected unified loop setting");
	}

	/* the trailing space is apparently important here */
	if (vm_rpc_send_rpci_tx("tools.capability.statechange ") != 0) {
		err_print("unable to send statechange capability");
		rpc->sc_rpc_error = 1;
	}

	if (vm_rpci_response_successful() == 0) {
		err_print("host rejected statechange capability");
	}

	if (vm_rpc_send_rpci_tx("tools.set.version %u", VM_VERSION_UNMANAGED) != 0) {
		err_print("unable to set tools version");
		rpc->sc_rpc_error = 1;
	}

	if (vm_rpc_send_str(rpc, (const uint8_t *)VM_RPC_REPLY_OK) != 0) {
		err_print("error sending capabilities_register"
			  " response");
		rpc->sc_rpc_error = 1;
	}
}

static void vmt_tclo_state_change_success(int success, char state)
{
	if (vm_rpc_send_rpci_tx("tools.os.statechange.status %d %d", success, state) != 0) {
		err_print("unable to send state change result");
		rpc->sc_rpc_error = 1;
	}
}

static void vmt_tclo_poweron(void)
{
	vmt_tclo_state_change_success(1, VM_STATE_CHANGE_POWERON);

	// now it is up, we can slow down
	fdelay = LOOP_WAIT_MS;

	if (vm_rpc_send_str(rpc, (const uint8_t *)VM_RPC_REPLY_OK) != 0) {
		err_print("error sending poweron response");
		rpc->sc_rpc_error = 1;
	}
}

static int network_ioctl(unsigned long req, void *data)
{
	int sockfd, err = 0;

	if ((sockfd = socket(AF_INET, SOCK_DGRAM | SOCK_CLOEXEC, 0)) < 0) {
		return sockfd;
	}

	err = ioctl(sockfd, req, data);

	close(sockfd);

	return err;
}

static void vmt_tclo_ping(void)
{
	vmt_update_guest_uptime();
	vmt_update_guest_info();

	if (vm_rpc_send_str(rpc, (const uint8_t *)VM_RPC_REPLY_OK) != 0) {
		err_print("error sending ping response");
		rpc->sc_rpc_error = 1;
	}
}

static void vmt_update_guest_info(void)
{
	if (vm_rpc_send_rpci_tx("SetGuestInfo  %d %s", VM_GUEST_INFO_DNS_NAME, hostname) != 0) {
		err_print("unable to set hostname");
		rpc->sc_rpc_error = 1;
	}

	// should be xdr but we limit it to 4 cards and send the info we have
	switch (numberOfCards) {
	case 2: {
		set_network_info(0, (char *)&card2);
		set_network_info(1, (char *)&card2);
		memcpy(rpc->sc_rpc_buf, &card2, card2_len);
		vm_rpc_send_rpci_tx_buf(rpc, (const uint8_t *)rpc->sc_rpc_buf, card2_len);
		break;
	}
	case 3: {
		set_network_info(0, (char *)&card3);
		set_network_info(1, (char *)&card3);
		set_network_info(2, (char *)&card3);
		memcpy(rpc->sc_rpc_buf, &card3, card3_len);
		vm_rpc_send_rpci_tx_buf(rpc, (const uint8_t *)rpc->sc_rpc_buf, card3_len);
		break;
	}
	case 4: {
		set_network_info(0, (char *)&card4);
		set_network_info(1, (char *)&card4);
		set_network_info(2, (char *)&card4);
		set_network_info(3, (char *)&card4);
		memcpy(rpc->sc_rpc_buf, &card4, card4_len);
		vm_rpc_send_rpci_tx_buf(rpc, (const uint8_t *)rpc->sc_rpc_buf, card4_len);
		break;
	}
	case 1:
	default: {
		// one or more that 4, we report one
		set_network_info(0, (char *)&card1);
		memcpy(rpc->sc_rpc_buf, &card1, card1_len);
		vm_rpc_send_rpci_tx_buf(rpc, (const uint8_t *)rpc->sc_rpc_buf, card1_len);
		break;
	}
	}

	if (rpc->sc_set_guest_os == 0) {
		if (vm_rpc_send_rpci_tx("SetGuestInfo  %d %s %s %s", VM_GUEST_INFO_OS_NAME_FULL, "vorteil.io", "1.0", "amd64_x86") != 0) {
			err_print("unable to set full guest OS");
			rpc->sc_rpc_error = 1;
		}

		if (vm_rpc_send_rpci_tx("SetGuestInfo  %d %s", VM_GUEST_INFO_OS_NAME, "other-64") != 0) {
			err_print("unable to set guest OS");
			rpc->sc_rpc_error = 1;
		}
		rpc->sc_set_guest_os = 1;
	}
}

static void vmt_tclo_broadcastip(void)
{
	struct ifreq ir;
	char ip[128];
	char *name = "eth0";

	memset(&ir, 0, sizeof ir);
	strncpy(ir.ifr_name, name, IFNAMSIZ);

	if (network_ioctl(SIOCGIFADDR, &ir)) {
		return;
	}

	inet_ntop(AF_INET, &(((struct sockaddr_in *)&ir.ifr_addr)->sin_addr), ip, sizeof(ip));

	if (vm_rpc_send_rpci_tx("info-set guestinfo.ip %s", ip) != 0) {
		err_print("unable to send guest IP address");
		rpc->sc_rpc_error = 1;
	}

	if (vm_rpc_send_str(rpc, (const uint8_t *)VM_RPC_REPLY_OK) != 0) {
		err_print("error sending broadcastIP response");
		rpc->sc_rpc_error = 1;
	}
}

static void vmt_update_guest_uptime(void)
{
	if (vm_rpc_send_rpci_tx("SetGuestInfo  %d %lld00", VM_GUEST_INFO_UPTIME, (long long)UptimeForTools()) != 0) {
		err_print("unable to set guest uptime");
		rpc->sc_rpc_error = 1;
	}
}

static void set_network_info(int card_no, char *msg)
{
	char name[10], mac_address[6], mac[64];
	int i = 0, n, base_mac, base_ip, base_prefix;
	uint64_t m, b, p;

	struct ifreq ir;

	snprintf(name, 10, "eth%d", card_no);

	memset(&ir, 0, sizeof ir);
	strncpy(ir.ifr_name, name, IFNAMSIZ);

	if (network_ioctl(SIOCGIFHWADDR, &ir)) {
		return;
	}

	memcpy(mac_address, ir.ifr_hwaddr.sa_data, 6);
	snprintf((char *)&mac, 64, "%02x:%02x:%02x:%02x:%02x:%02x", mac_address[0], mac_address[1], mac_address[2], mac_address[3], mac_address[4],
		 mac_address[5]);

	// byte where mac addr is
	// 112 is the offset between them
	base_mac = 32;
	base_ip = 64;
	base_prefix = 71;

	if (network_ioctl(SIOCGIFNETMASK, &ir)) {
		return;
	}

	// get prefix length
	n = ((struct sockaddr_in *)&ir.ifr_addr)->sin_addr.s_addr;
	while (n > 0) {
		n = n >> 1;
		i++;
	}

	if (network_ioctl(SIOCGIFADDR, &ir)) {
		return;
	}

	m = (uint64_t)msg + base_mac + (card_no * 72);
	b = (uint64_t)msg + base_ip + (card_no * 72);
	p = (uint64_t)msg + base_prefix + (card_no * 72);

	memcpy((uintptr_t *)m, &mac, 17);
	memcpy((uintptr_t *)b, &(((struct sockaddr_in *)&ir.ifr_addr)->sin_addr.s_addr), sizeof(uint32_t));
	memcpy((uintptr_t *)p, &i, sizeof(uint32_t));
}

static void vmt_tclo_halt(void)
{
	vmt_tclo_state_change_success(1, VM_STATE_CHANGE_HALT);
	vm_rpc_send_str(rpc, (const uint8_t *)VM_RPC_REPLY_OK);
	ShutdownForTools();
}

static void vmt_tclo_reboot(void)
{
	vmt_tclo_state_change_success(1, VM_STATE_CHANGE_REBOOT);
	vm_rpc_send_str(rpc, (const uint8_t *)VM_RPC_REPLY_OK);
	RebootForTools();
}

static void vmt_tclo_resume(void)
{
	vmt_update_guest_info();

	vmt_tclo_state_change_success(1, VM_STATE_CHANGE_RESUME);
	if (vm_rpc_send_str(rpc, (const uint8_t *)VM_RPC_REPLY_OK) != 0) {
		err_print("error sending resume response");
		rpc->sc_rpc_error = 1;
	}
}

static void vmt_tclo_suspend(void)
{
	vmt_tclo_state_change_success(1, VM_STATE_CHANGE_SUSPEND);
	if (vm_rpc_send_str(rpc, (const uint8_t *)VM_RPC_REPLY_OK) != 0) {
		err_print("error sending suspend response");
		rpc->sc_rpc_error = 1;
	}
}
