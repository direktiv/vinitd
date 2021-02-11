/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

#ifndef _HELPER_H
#define _HELPER_H

extern int helper_add_route(int dst, int mask, int addr, char *dev, int flags);

extern int helper_add_gcp_virtual_route(char *dev, char *ip);

#endif
