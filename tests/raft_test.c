#include <stdio.h>

void RAFTVersion(char* version, size_t n);

int main() {
    char ver[1024];
    RAFTVersion(ver, 1024);
    while(1) {
        printf("%s\n", ver);
    }
    return 0;
}