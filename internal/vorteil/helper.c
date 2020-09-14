/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */
 
#include <stdio.h>
#include <errno.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <pthread.h>
#include <stdarg.h>
#include <signal.h>

#include <sys/socket.h>
#include <net/route.h>
#include <sys/ioctl.h>
#include <arpa/inet.h>
#include <linux/sockios.h>
#include <sys/syscall.h>

static inline void set_sockaddr(struct sockaddr_in *sin, int addr)
{
	sin->sin_family = AF_INET;
	sin->sin_addr.s_addr = addr;
	sin->sin_port = 0;
}

int helper_add_route(int dst, int mask, int gw, char *dev, int flags)
{
	struct rtentry *rm;
	int err, fd;

	rm = calloc(1, sizeof(struct rtentry));

	if (dev)
		rm->rt_dev = dev;

	set_sockaddr((struct sockaddr_in *)&rm->rt_dst, dst);
	set_sockaddr((struct sockaddr_in *)&rm->rt_genmask, mask);
	set_sockaddr((struct sockaddr_in *)&rm->rt_gateway, gw);

	rm->rt_flags = flags;

	fd = socket(PF_INET, SOCK_DGRAM | SOCK_CLOEXEC, IPPROTO_IP);
	if (fd < 0) {
		fprintf(stderr, "can not add route: %s\n", strerror(errno));
		err = 1;
		goto ret;
	}

	err = ioctl(fd, SIOCADDRT, rm);
	if (err) {
		fprintf(stderr, "can not add route: %s\n", strerror(errno));
	}
	close(fd);

ret:;
	free(rm);
	return err;
}

void err_print(char *txt)
{
	FILE *fp;
	fp = fopen("/dev/vtty", "w+");
	fprintf(fp, "%s\n", txt);
	fclose(fp);
}
