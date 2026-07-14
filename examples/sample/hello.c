/* Минимальная программа для демонстрации debuginfod-go + GDB. */
#include <stdio.h>

static int add(int a, int b)
{
	return a + b;
}

void greet(const char *name)
{
	int answer = add(40, 2);
	printf("Hello, %s! answer=%d\n", name, answer);
}

int main(int argc, char **argv)
{
	const char *who = (argc > 1) ? argv[1] : "debuginfod";
	greet(who);
	return 0;
}
