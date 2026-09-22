#!/usr/bin/env python3
"""Assert a rendered SVG contains every node label from its d2 source.

Layout differs between the oracle (dagre) and the embedded engine (TALA), so
the bytes cannot be compared; the labels can.

    parity-labels.py <svg> <d2 source>
"""
import re
import sys

svg = open(sys.argv[1]).read()
src = open(sys.argv[2]).read()
labels = set(re.findall(r'label: "([^"]+)"', src))
labels |= set(re.findall(r'^"([^"]+)": \{$', src, re.M))
# a d2 label's "\n" is a line break; the SVG splits it across tspans
missing = sorted(l for l in labels if l.split(chr(92))[0] not in svg)
if missing:
    print("missing from the SVG:", missing, file=sys.stderr)
    sys.exit(1)
