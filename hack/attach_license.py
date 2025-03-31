#!/usr/bin/env python3

"""
This script is used to attach apache license to all source files.
"""

import os
import re

with open('./hack/boilerplate.go.txt') as f:
    AUTH_COMMENT = f.read()

PATTERN = r"/\*.*?Licensed under the Apache License.*?\*/\n?"
EXTENSION = ".go"

for root, dirs, files in os.walk('.'):
    for file in files:
        if file.endswith(EXTENSION):
            file_path = os.path.join(root, file)
            with open(file_path, 'r+') as f:
                content = f.read()
                if re.search(PATTERN, content, re.DOTALL):
                    # Replace existing block comment
                    content = re.sub(PATTERN, AUTH_COMMENT, content, flags=re.DOTALL)
                else:
                    # Prepend new comment
                    content = AUTH_COMMENT + '\n' + content
                f.seek(0)
                f.write(content)
                f.truncate()
