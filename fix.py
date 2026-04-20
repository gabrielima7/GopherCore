import re

with open('config/config_test.go', 'r') as f:
    content = f.read()

# remove unused "os" import
content = re.sub(r'\t"os"\n', '', content)

with open('config/config_test.go', 'w') as f:
    f.write(content)
