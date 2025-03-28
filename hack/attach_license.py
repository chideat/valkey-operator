#!/usr/bin/env python3

"""
This script is used to attach apache license to all source files.
"""

import os
import re

with open('./hack/boilerplate.go.txt') as f:
    AUTH_COMMENT = f.read()

EXTENSION = ".go"

for root, dirs, files in os.walk('.'):
    for file in files:
        if file.endswith(EXTENSION):
            file_path = os.path.join(root, file)
            with open(file_path, 'r+') as f:
                content = f.read()
                index = content.index("package ")
                if index == 0:
                    content = AUTH_COMMENT + '\n' + content
                else:
                    content = AUTH_COMMENT + '\n' + content[index:]
                f.seek(0)
                f.write(content)
                f.truncate()
