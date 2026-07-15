import re
c=open('global-fix.css','r',encoding='utf-8').read()

# Fix pattern: .reader-md, .blog-content ELEMENT → .reader-md ELEMENT, .blog-content ELEMENT
# This matches: .reader-md, .blog-content h1, .reader-md, .blog-content p, etc.
def fix_selector(m):
    elem = m.group(2)  # e.g. "h1", "p", ".katex", "table", etc.
    return f'.reader-md {elem},\n.blog-content {elem}'

c = re.sub(r'\.reader-md,\s*\.blog-content\s+(h[1-6]|p\b|dl|dt|dd|table\b|th\b|td\b|abbr|pre|code|\.katex|\.mermaid)', fix_selector, c)

# Fix dark theme pattern: [data-theme="dark"] .reader-md, [data-theme="dark"] .blog-content, .blog-content ELEMENT
def fix_dark_selector(m):
    elem = m.group(1)
    return f'[data-theme="dark"] .reader-md {elem},\n[data-theme="dark"] .blog-content {elem}'

c = re.sub(r'\[data-theme="dark"\]\s+\.reader-md,\s*\[data-theme="dark"\]\s+\.blog-content,\s*\.blog-content\s+(h[1-6]|p\b|dl|dt|dd|table\b|th\b|td\b|abbr|pre|code|\.katex|\.mermaid)', fix_dark_selector, c)

# Also fix simpler dark theme: [data-theme="dark"] .reader-md, [data-theme="dark"] .blog-content ELEMENT
def fix_dark2(m):
    elem = m.group(1)
    return f'[data-theme="dark"] .reader-md {elem},\n[data-theme="dark"] .blog-content {elem}'

c = re.sub(r'\[data-theme="dark"\]\s+\.reader-md,\s*\[data-theme="dark"\]\s+\.blog-content\s+(h[1-6]|p\b|dl|dt|dd|table\b|th\b|td\b|abbr|pre|code|\.katex|\.mermaid)', fix_dark2, c)

# Verify no broken selectors remain
broken = re.findall(r'\.reader-md,\s*\.blog-content\s+(h[1-6]|p\b|dl|dt|dd|table\b|th\b|td\b|abbr|pre|code|\.katex|\.mermaid)', c)
print(f'Remaining broken: {len(broken)}')
if broken:
    for b in broken:
        print(f'  - .reader-md, .blog-content {b}')

open('global-fix.css','w',encoding='utf-8').write(c)
print('Done')
