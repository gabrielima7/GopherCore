import re

with open('config/config_test.go', 'r') as f:
    content = f.read()

content = content.replace(
    'if len(cfg.SliceVal) != 2 || cfg.SliceVal[0] != 1 || cfg.SliceVal[1] != 2 {\n\t\tt.Errorf("expected SliceVal [1 2], got %v", cfg.SliceVal)\n\t}',
    'if len(cfg.SliceVal) != 2 || cfg.SliceVal[0] != 1 || cfg.SliceVal[1] != 2 {\n\t\tt.Errorf("expected SliceVal [1 2], got %v", cfg.SliceVal)\n\t}\n\t_ = cfg.unexported'
)

with open('config/config_test.go', 'w') as f:
    f.write(content)
