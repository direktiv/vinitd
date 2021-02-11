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
#include <net/if.h>
#include <sys/ioctl.h>
#include <arpa/inet.h>
#include <linux/sockios.h>
#include <linux/rtnetlink.h>
#include <linux/genetlink.h>
#include <sys/syscall.h>

static inline void set_sockaddr(struct sockaddr_in *sin, int addr)
{
	sin->sin_family = AF_INET;
	sin->sin_addr.s_addr = addr;
	sin->sin_port = 0;
}

#define NLA_DATA(na) ((void *)((char*)(na) + NLA_HDRLEN))
#define GENLMSG_DATA(glh) ((void *)(NLMSG_DATA(glh) + GENL_HDRLEN))

int helper_add_gcp_virtual_route(char *dev, char *ip)
{
  
  struct sockaddr_nl saddr;
  struct nlattr *nl_na;

  struct {
      struct nlmsghdr n;
      struct rtmsg r;
      char buf[4096];
  } nl_request;

  int sock, ret, idx;
  in_addr_t addr;

   sock = socket(AF_NETLINK, SOCK_RAW, NETLINK_ROUTE);
  if (sock < 0) {
      return 1;
  }
  memset(&saddr, 0, sizeof(saddr));

  nl_request.n.nlmsg_len = NLMSG_LENGTH(sizeof(struct rtmsg) + 16);

  nl_request.n.nlmsg_flags = NLM_F_REQUEST | NLM_F_ACK|NLM_F_EXCL|NLM_F_CREATE;
  nl_request.n.nlmsg_type = RTM_NEWROUTE;

  nl_request.r.rtm_family = AF_INET;
  nl_request.r.rtm_table = RT_TABLE_LOCAL;
  nl_request.r.rtm_src_len = 0;
  nl_request.r.rtm_tos = 0;
  nl_request.r.rtm_dst_len = 32;
  nl_request.r.rtm_scope = RT_SCOPE_HOST;
  nl_request.r.rtm_protocol = 0x42;
  nl_request.r.rtm_type = RTN_LOCAL;
  nl_request.r.rtm_flags = 0;

  nl_na = (struct nlattr *) &nl_request.buf;
  nl_na->nla_type = RTA_DST;
  nl_na->nla_len = 8;

  addr = inet_addr(ip);
  memset(NLA_DATA(nl_na), 0, 8);
  memcpy(NLA_DATA(nl_na), &addr, sizeof(addr));

  nl_na = (struct nlattr *) ((char *) nl_na + NLA_ALIGN(nl_na->nla_len));
  nl_na->nla_type = RTA_OIF;
  nl_na->nla_len = 8;

  memset(NLA_DATA(nl_na), 0, 8);
  idx = if_nametoindex(dev);
  memcpy(NLA_DATA(nl_na), &idx, sizeof(idx));

  ret =  send(sock, &nl_request, sizeof(nl_request), 0);

  close (sock);

  return ret;

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
