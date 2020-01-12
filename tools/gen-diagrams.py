#!/usr/bin/env python3

import subprocess

diagrams = [
    {
        'source': 'pkg/defkinds/php/testdata/build/state-dev.json',
        'dest': 'docs/diagrams/php-stage-dev.png',
    },
    {
        'source': 'pkg/defkinds/php/testdata/build/state-prod.json',
        'dest': 'docs/diagrams/php-stage-prod.png',
    },
]

for diagram in diagrams:
    cmd = ("cat {source} | "+
           "zbuild llbgraph | "+
           "dot /dev/stdin -o {dest} -T png").format(**diagram)
    
    subprocess.run(cmd, shell=True, check=True)
