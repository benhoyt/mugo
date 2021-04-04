#include <stdio.h>
#include <string.h>
#include <stdlib.h>

typedef struct {
    char* str;
    int len;
} string;

void* _allocChecked(size_t n) {
    void* mem = malloc(n);
    if (mem == NULL) {
        fprintf(stderr, "out of memory\n");
        exit(1);
    }
    return mem;
}

string _strAdd(string a, string b) {
    int totalLen = a.len + b.len;
    string result = {_allocChecked(totalLen), totalLen};
    memcpy(result.str, a.str, a.len);
    memcpy(result.str + a.len, b.str, b.len);
    return result;
}

void main() {
    string x = {"Hello ", 6};
    string y = {"world!!!", 8};
    string z = _strAdd(x, y);
    printf("%.*s\n", z.len, z.str);
}