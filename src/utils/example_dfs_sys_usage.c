/**
 * (C) Copyright 2016-2021 Intel Corporation.
 *
 * SPDX-License-Identifier: BSD-2-Clause-Patent
 */

/* example_dfs_sys_usage.c - Example usage for DFS Sys API
 */

#include <fcntl.h>
#include <stdio.h>

#include <daos.h>
#include <daos_fs_sys.h>

int main(int argc, char **argv)
{
	int              rc = 0;
	char             pool[DAOS_PROP_LABEL_MAX_LEN + 1];
	char             cont[DAOS_PROP_LABEL_MAX_LEN + 1];
	daos_handle_t    poh       = DAOS_HDL_INVAL;
	daos_handle_t    coh       = DAOS_HDL_INVAL;
	daos_pool_info_t pool_info = {0};
	daos_cont_info_t cont_info = {0};
	dfs_sys_t *      dfs_sys   = NULL;
	char *           file_path = "/test_file";
	dfs_obj_t *      file_obj  = NULL;
	daos_size_t      buf_size  = 10;
	daos_size_t      io_size;
	void *           write_buf[buf_size];
	void *           read_buf[buf_size];

	/** Simple pool/cont arguments */
	if (argc < 3) {
		printf("Usage: %s <pool> <cont>\n", argv[0]);
		return 1;
	}

	strcpy(pool, argv[1]);
	strcpy(cont, argv[2]);

	printf("pool: %s\n", pool);
	printf("cont: %s\n", cont);

	/** Standard daos_init, pool_connect, cont_open */
	rc = daos_init();
	if (rc) {
		printf("daos_init failed " DF_RC "\n", DP_RC(rc));
		goto out;
	}

	rc = daos_pool_connect(pool, NULL, DAOS_PC_RW, &poh, &pool_info, NULL);
	if (rc) {
		printf("daos_pool_connect failed " DF_RC "\n", DP_RC(rc));
		goto out;
	}

	rc = daos_cont_open(poh, cont, DAOS_COO_RW, &coh, &cont_info, NULL);
	if (rc) {
		printf("daos_cont_open failed " DF_RC "\n", DP_RC(rc));
		goto out;
	}

	/** Mount DFS Sys. Similar to DFS. Requires pool and container handles. */
	rc = dfs_sys_mount(poh, coh, O_RDWR, 0, &dfs_sys);
	if (rc) {
		printf("dfs_sys_mount failed %s (%d)\n", strerror(rc), (rc));
		goto out;
	}

	/** Open and create a file in the container.
	 * Similar to open(file_path, O_RDWR | O_CREAT, S_IFREG | S_IWUSR | S_IRUSR) */
	rc = dfs_sys_open(dfs_sys, file_path, S_IFREG | S_IWUSR | S_IRUSR, O_RDWR | O_CREAT, 0, 0,
			  NULL, &file_obj);
	if (rc) {
		printf("dfs_sys_open failed %s (%d)\n", strerror(rc), (rc));
		goto out;
	}

	/** Set IO size, buffer data, and write to file.
	 * Similar to pwrite(fd, write_buf, io_size, 0) */
	io_size = buf_size;
	memset(write_buf, '1', buf_size);
	rc = dfs_sys_write(dfs_sys, file_obj, write_buf, 0, &io_size, NULL);
	if (rc) {
		printf("dfs_sys_write failed %s (%d)\n", strerror(rc), (rc));
		goto out;
	};
	printf("Wrote %lu/%lu bytes\n", io_size, buf_size);

	/** Reset IO size, though all bytes were written */
	io_size = buf_size;

	/** Read specified IO size.
	 * Similar to pread(fd, read_buf, io_size, 0) */
	rc = dfs_sys_read(dfs_sys, file_obj, read_buf, 0, &io_size, NULL);
	if (rc) {
		printf("dfs_sys_read failed %s (%d)\n", strerror(rc), (rc));
		goto out;
	}
	printf("Read %lu/%lu bytes\n", io_size, buf_size);

	/** Close the file handle.
	 * Similar to close(fd) */
	rc = dfs_sys_close(file_obj);
	if (rc) {
		printf("dfs_sys_close failed %s (%d)\n", strerror(rc), (rc));
		goto out;
	}
	file_obj = NULL;

	/** Delete the file.
	 * Similar to unlink(file_path) */
	rc = dfs_sys_remove(dfs_sys, file_path, false, NULL);
	if (rc) {
		printf("dfs_sys_remove failed %s (%d)\n", strerror(rc), (rc));
		goto out;
	}

	/** Unmound DFS Sys */
	rc = dfs_sys_umount(dfs_sys);
	if (rc) {
		printf("dfs_sys_umount failed %s (%d)\n", strerror(rc), (rc));
		goto out;
	}
	dfs_sys = NULL;

	/** Standard daos_cont_close, daos_pool_disconnect */
	rc = daos_cont_close(coh, NULL);
	if (rc) {
		printf("daos_cont_close failed " DF_RC "\n", DP_RC(rc));
		goto out;
	}
	coh = DAOS_HDL_INVAL;

	rc = daos_pool_disconnect(poh, NULL);
	if (rc) {
		printf("daos_pool_disconnect failed " DF_RC "\n", DP_RC(rc));
		goto out;
	}
	poh = DAOS_HDL_INVAL;

out:
	/** Simple cleanup in error cases */
	if (file_obj != NULL)
		dfs_sys_close(file_obj);
	dfs_sys_remove(dfs_sys, file_path, true, NULL);
	if (dfs_sys != NULL)
		dfs_sys_umount(dfs_sys);
	if (daos_handle_is_valid(coh))
		daos_cont_close(coh, NULL);
	if (daos_handle_is_valid(poh))
		daos_pool_disconnect(poh, NULL);

	/** Simple 0/1 error */
	if (rc)
		return 1;
	return 0;
}