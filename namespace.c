#include "namespace.h"

#define _GNU_SOURCE
#include <sched.h>
#include <linux/sched.h>
#include <stdint.h>
#include <unistd.h>
#include <string.h>
#include <stdio.h>
#include <errno.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <sys/wait.h>
#include <stdlib.h>

#define E9 1000000000
#define WAIT_TIME   60000000

int setns(int fd, int nstype);

static void wait_pid(int pid, int microseconds) {
    int elapse = 0;
    fprintf(stderr, "pid: %d\n", pid);
    while(1) {
        int res = waitpid(pid, NULL, WNOHANG);
        if (res == -1 || res == pid) {
            if (errno == EINTR) {
                errno = 0;
            }
            break;
        }

        usleep(100000);
        elapse += 100000;
        if (elapse > microseconds) {
            kill(pid, SIGKILL);
            break;
        }
    }
}

const char* ns_read(const char *filename, const char *path, int *err) {
    int fds[2];
    if (-1 == pipe(fds)) {
        *err = errno;
        return NULL;
    }

    pid_t pid = fork();
    if (-1 == pid) {
        *err = errno;
        return NULL;
    } else if (0 == pid) {
        close(fds[0]);
        int nfd = open(path, O_RDONLY | O_CLOEXEC);
        char buf[1024] = {0};
        if (-1 == nfd) {
            goto error;
        }

        if (-1 == setns(nfd, 0)) {
            goto error;
        }

        struct stat statbuf;
        if (-1 == stat(filename, &statbuf)) {
            goto error;
        }

        int res;
        ((int*)(buf))[0] = 0;
        ((int*)(buf))[1] = (int)statbuf.st_size;
        res = write(fds[1], buf, sizeof(int)*2);

        int fd = open(filename, O_RDONLY);
        if (-1 == fd) {
            goto error;
        }

        while (1) {
            int size = read(fd, buf, 1024);
            if (size <= 0) {
                break;
            }

            res = write(fds[1], buf, size);
        }

        close(nfd);
        close(fds[1]);
        close(fd);
        exit(0);

      error:
        ((int*)(buf))[0] = errno;
        res = write(fds[1], buf, sizeof(int));
        close(fds[1]);
        exit(1);
    }
    char *buf = NULL;
    close(fds[1]);
    char data[1024];
    if (-1 == fcntl(fds[0], F_SETFL, O_NONBLOCK)) {
        *err = errno;
        goto exit;
    }

    int left = sizeof(int) * 2;
    int offset = 0;
    int elapse = 0;
    while (left > 0) {
        int size = read(fds[0], data + offset, left);
        if (size == -1) {
            if (errno == EAGAIN) {
                usleep(10000);
                elapse += 10000;
                if (elapse > 20000000) {
                    *err = ETIME;
                    goto exit;
                }
                continue;
            }

            *err = errno;
            goto exit;
        }

        offset += size;
        left -= size;
    }

    *err = ((int*)(data))[0];
    if (*err != 0) {
        goto exit;
    }

    int size = ((int*)(data))[1];
    if (size <= 0) {
        goto exit;
    }

    buf = malloc(size);
    if (NULL == buf) {
        *err = errno;
        goto exit;
    }

    left = size;
    offset = 0;
    elapse = 0;
    while (left > 0) {
        int n = read(fds[0], data, left > 1024 ? 1024 : left);
        if (-1 == n) {
            if (errno == EAGAIN) {
                usleep(10000);
                elapse += 10000;

                if (elapse > 120000000) {
                    *err = ETIME;
                    free(buf);
                    buf = NULL;
                    goto exit;
                }

                continue;
            }

            *err = errno;
            free(buf);
            buf = NULL;
            break;
        }

        memcpy(buf + offset, data, n);
        offset += n;
        left -= n;
    }

    buf[offset] = 0;

exit:
    wait_pid(pid, WAIT_TIME);
    close(fds[0]);

    return buf;
}

const int ns_stat(const char *filename, const char *path, void *userdata) {
    if (NULL == stat) {
        return EINVAL;
    }

    struct stat *info = (struct stat*)(userdata);
    int fds[2];
    if (-1 == pipe(fds)) {
        return errno;
    }

    pid_t pid = fork();
    if (-1 == pid) {
        return errno;
    } else if (0 == pid) {
        close(fds[0]);
        int nfd = open(path, O_RDONLY | O_CLOEXEC);
        int64_t buf[11] = {0};
        if (-1 == nfd) {
            goto error;
        }

        if (-1 == setns(nfd, 0)) {
            goto error;
        }

        struct stat statbuf;
        if (-1 == stat(filename, &statbuf)) {
            goto error;
        }

        buf[0] = 0;
        buf[1] = statbuf.st_uid;
        buf[2] = statbuf.st_gid;
        buf[3] = statbuf.st_size;
        buf[4] = statbuf.st_mode;
        buf[5] = statbuf.st_ino;
        buf[6] = statbuf.st_blksize;
        buf[7] = statbuf.st_blocks;
        buf[8] = statbuf.st_nlink;
        buf[9] = statbuf.st_atim.tv_sec * E9 + statbuf.st_atim.tv_nsec;
        buf[10] = statbuf.st_mtim.tv_sec * E9 + statbuf.st_mtim.tv_nsec;

        int res = write(fds[1], (char*)buf, sizeof(int64_t) * 11);

        close(nfd);
        close(fds[1]);
        exit(0);

      error:
        buf[0] = errno;
        res = write(fds[1], buf, sizeof(int64_t));
        close(fds[1]);
        exit(1);
    }

    close(fds[1]);
    int64_t buf[11] = {0};

    int left = sizeof(int64_t) * 11;
    int offset = 0;
    if (-1 == fcntl(fds[0], F_SETFL, O_NONBLOCK)) {
        goto exit;
    }

    int elapse = 0;
    while (left > 0) {
        int n = read(fds[0], (char*)buf + offset, left);
        if (-1 == n) {
            if (errno == EAGAIN) {
                usleep(10000);
                elapse += 10000;
                if (elapse > 30000000) {
                    errno = ETIME;
                    goto exit;
                }
                continue;
            }
            break;
        }

        offset += n;
        left -= n;
    }

    errno = buf[0];
    info->st_uid = buf[1];
    info->st_gid = buf[2];
    info->st_size = buf[3];
    info->st_mode = buf[4];
    info->st_ino = buf[5];
    info->st_blksize = buf[6];
    info->st_blocks = buf[7];
    info->st_nlink = buf[8];
    info->st_atim.tv_sec = buf[9] / E9;
    info->st_atim.tv_nsec = buf[9] % E9;
    info->st_mtim.tv_sec = buf[10] / E9;
    info->st_mtim.tv_nsec = buf[10] % E9;

exit:
    wait_pid(pid, WAIT_TIME);
    close(fds[0]);

    return errno;
}
