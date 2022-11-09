#ifndef __NAMESPACE_H__
#define __NAMESPACE_H__

#include <stdint.h>

const char* ns_read(const char *filename, const char *path, int *err);
const int ns_stat(const char *filename, const char *path, void *stat);

#endif // __NAMESPACE_H__
