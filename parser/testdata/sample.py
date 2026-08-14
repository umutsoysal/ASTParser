import os
import sys
from collections import OrderedDict


class Greeter:
    """Says hello."""

    def __init__(self, name):
        self.name = name

    def greet(self):
        return f"hello, {self.name}"


def main():
    counts = OrderedDict()
    counts[os.name] = len(sys.argv)
    print(Greeter("world").greet(), counts)


if __name__ == "__main__":
    main()
