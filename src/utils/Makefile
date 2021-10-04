
GCC=gcc -Wall -Wno-missing-braces

DAOS_INCLUDE_DIR=/home/dbohning/daos/install/include
DAOS_LIB_DIR=/home/dbohning/daos/install/lib64/

INCLUDE_FLAGS=-I${DAOS_INCLUDE_DIR}
LDFLAGS=-L${DAOS_LIB_DIR} -luuid -ldaos -ldfs -ldaos_common -lgurt -lpthread

default: clean example

example: example_dfs_sys_usage.c
	${GCC} ${INCLUDE_FLAGS} ${LDFLAGS} -o example example_dfs_sys_usage.c

clean:
	rm -f example
