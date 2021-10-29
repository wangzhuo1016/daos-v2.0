#!/bin/bash
# Copyright (C) Copyright 2020 Intel Corporation
# All rights reserved.
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted for any purpose (including commercial purposes)
# provided that the following conditions are met:
#
# 1. Redistributions of source code must retain the above copyright notice,
#    this list of conditions, and the following disclaimer.
#
# 2. Redistributions in binary form must reproduce the above copyright notice,
#    this list of conditions, and the following disclaimer in the
#    documentation and/or materials provided with the distribution.
#
# 3. In addition, redistributions of modified forms of the source or binary
#    code must carry prominent notices stating that the original code was
#    changed and the date of the change.
#
#  4. All publications or advertising materials mentioning features or use of
#     this software are asked, but not required, to acknowledge that it was
#     developed by Intel Corporation and credit the contributors.
#
# 5. Neither the name of Intel Corporation, nor the name of any Contributor
#    may be used to endorse or promote products derived from this software
#    without specific prior written permission.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
# AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
# IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
# ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER BE LIABLE FOR ANY
# DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
# (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
# LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
# ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
# (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF
# THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

set -eux
# allow core files to be generated
sudo bash -c "set -ex
if [ \"\$(ulimit -c)\" != \"unlimited\" ]; then
    echo \"*  soft  core  unlimited\" >> /etc/security/limits.conf
fi
echo \"/var/tmp/core.%e.%t.%p\" > /proc/sys/kernel/core_pattern"
sudo rm -f /var/tmp/core.*
if [ "${HOSTNAME%%.*}" != "$FIRST_NODE" ]; then
    if /sbin/lspci | grep "Non-Volatile memory controller"; then
        if /sbin/lspci | grep -e "Non-Volatile memory controller: Intel Corporation QEMU NVM Express Controller" \
                              -e "Non-Volatile memory controller: Red Hat, Inc. QEMU NVM Express Controller"     \
                              -e "Non-Volatile memory controller: Red Hat, Inc. Device 0010"; then
            sudo bash -c "set -ex
# VMs don't have IOMMU (except https://wiki.qemu.org/Features/VT-d)
mkdir -p /etc/systemd/system/daos_server.service.d/
cat <<EOF > /etc/systemd/system/daos_server.service.d/override.conf
[Service]
User=root
Group=root
EOF

create_namespaces() {
    for x in 0 1; do
        if ! ndctl destroy-namespace -f namespace\${x}.0; then
            echo \"Failed to destroy namespaces\"
            exit 1
        fi
    done
    ndctl list -Nu || true
    created=0
    for x in 0 1; do
        if ndctl create-namespace -f; then
            (( created++ )) || true
        fi
    done
    ndctl list -Nu || true

    echo $\created
}

ls -l /dev/pmem*
ndctl list --regions
ndctl list -Nu
ndctl disable-region all || true
ndctl init-labels -f all || true
ndctl enable-region all || true
ndctl list -Nu || true
modified=0
ndctl list -Ni || true
if [[ $(lsb_release -s -r) = 8.* ]]; then
    for x in 0 1; do
        if ! ndctl create-namespace; then
            echo \"Failed to create namespace\"
            if ! ndctl create-namespace -e namespace\${x}.0 -m fsdax -f; then
                echo \"Failed to modify namespace\"
            else
                (( modified++ )) || true
            fi
        else
            (( modified++ )) || true
        fi
    done
    ndctl list -Nu || true
    if [ \"\$modified\" -lt 2 ]; then
        echo \"Failed to modify namespaces, trying to recreate them...\"
        created=\$(create_namespaces)
        if [ \"\$created\" -lt 2 ]; then
            echo \"Failed to create namespaces\"
            exit 1
        fi
    fi
else
    created=\$(create_namespaces)
    if [ \"\$created\" -lt 2 ]; then
        echo \"Failed to create namespaces\"
        exit 1
    fi
fi"
        fi
    else
        if grep /mnt/daos\  /proc/mounts; then
            sudo umount /mnt/daos
        else
            if [ ! -d /mnt/daos ]; then
                sudo mkdir -p /mnt/daos
            fi
        fi

        tmpfs_size=16777216
        memsize="$(sed -ne '/MemTotal:/s/.* \([0-9][0-9]*\) kB/\1/p' \
                   /proc/meminfo)"
        if [ "$memsize" -gt "32000000" ]; then
            # make it twice as big on the hardware cluster
            tmpfs_size=$((tmpfs_size*2))
        fi
        sudo ed <<EOF /etc/fstab
\$a
tmpfs /mnt/daos tmpfs rw,relatime,size=${tmpfs_size}k 0 0 # added by ftest.sh
.
wq
EOF
        sudo mount /mnt/daos
    fi
fi

rm -f /tmp/test.cov
if [ -f /usr/lib/daos/TESTING/ftest/test.cov ]; then
    cp /usr/lib/daos/TESTING/ftest/test.cov /tmp
    chmod 777 /tmp/test.cov
fi

# make sure to set up for daos_agent. The test harness will take care of
# creating the /var/run/daos_{agent,server} directories when needed.
sudo bash -c "set -ex
if [ -d  /var/run/daos_agent ]; then
    rm -rf /var/run/daos_agent
fi
if [ -d  /var/run/daos_server ]; then
    rm -rf /var/run/daos_server
fi
if $TEST_RPMS || [ -f $DAOS_BASE/SConstruct ]; then
    echo \"No need to NFS mount $DAOS_BASE\"
else
    mkdir -p $DAOS_BASE
    ed <<EOF /etc/fstab
\\\$a
$NFS_SERVER:$PWD $DAOS_BASE nfs defaults,vers=3 0 0 # DAOS_BASE # added by ftest.sh
.
wq
EOF
    mount \"$DAOS_BASE\"
fi"

# For verbs enable servers in dual-nic setups to talk to each other; no adverse effect for sockets
sudo sysctl -w net.ipv4.conf.all.accept_local=1
sudo sysctl -w net.ipv4.conf.all.arp_ignore=2
sudo sysctl -w net.ipv4.conf.all.rp_filter=2
find /sys/class/net/ -maxdepth 1 -name 'ib*' -print0 | xargs -t -I {} --null basename {} | \
    xargs -t -I {} sudo sysctl -w net.ipv4.conf.{}.rp_filter=2

if ! $TEST_RPMS; then
    # set up symlinks to spdk scripts (none of this would be
    # necessary if we were testing from RPMs) in order to
    # perform NVMe operations via daos_admin
    sudo mkdir -p /usr/share/daos/control
    sudo ln -sf "$SL_PREFIX"/share/daos/control/setup_spdk.sh \
               /usr/share/daos/control
    sudo mkdir -p /usr/share/spdk/scripts
    if [ ! -f /usr/share/spdk/scripts/setup.sh ]; then
        sudo ln -sf "$SL_PREFIX"/share/spdk/scripts/setup.sh \
                   /usr/share/spdk/scripts
    fi
    if [ ! -f /usr/share/spdk/scripts/common.sh ]; then
        sudo ln -sf "$SL_PREFIX"/share/spdk/scripts/common.sh \
                   /usr/share/spdk/scripts
    fi
    if [ ! -f /usr/share/spdk/include/spdk/pci_ids.h ]; then
        sudo rm -f /usr/share/spdk/include
        sudo ln -s "$SL_PREFIX"/include \
                   /usr/share/spdk/include
    fi

    # first, strip the execute bit from the in-tree binary,
    # then copy daos_admin binary into \$PATH and fix perms
    chmod -x "$DAOS_BASE"/install/bin/daos_admin && \
    sudo cp "$DAOS_BASE"/install/bin/daos_admin /usr/bin/daos_admin && \
	    sudo chown root /usr/bin/daos_admin && \
	    sudo chmod 4755 /usr/bin/daos_admin
fi

rm -rf "${TEST_TAG_DIR:?}/"
mkdir -p "$TEST_TAG_DIR/"
if [ -z "$JENKINS_URL" ]; then
    exit 0
fi
