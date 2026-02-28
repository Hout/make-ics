import re

def unfold(text):
    return re.sub(r'\r?\n[ \t]', '', text)

def parse_events(path):
    with open(path) as f:
        content = unfold(f.read())
    events = re.findall(r'BEGIN:VEVENT(.*?)END:VEVENT', content, re.DOTALL)
    parsed = []
    for ev in events:
        d = {}
        for line in ev.strip().splitlines():
            k, _, v = line.partition(':')
            k = k.split(';')[0].strip()
            d[k] = v.strip()
        parsed.append(d)
    return parsed

def parse_header(path):
    with open(path) as f:
        content = unfold(f.read())
    header = {}
    for line in content.splitlines():
        if line == 'BEGIN:VCALENDAR':
            continue
        if line == 'BEGIN:VEVENT':
            break
        k, _, v = line.partition(':')
        header[k.strip()] = v.strip()
    return header

report_hdr = parse_header('report.ics')
example_hdr = parse_header('example.ics')
all_hdr_keys = sorted(set(report_hdr) | set(example_hdr))
hdr_diffs = [(k, report_hdr.get(k), example_hdr.get(k)) for k in all_hdr_keys if report_hdr.get(k) != example_hdr.get(k)]
if hdr_diffs:
    print('=== Calendar header differences ===')
    for k, r, e in hdr_diffs:
        print(f'  {k}:')
        print(f'    REPORT:  {r}')
        print(f'    EXAMPLE: {e}')
    print()

report = parse_events('report.ics')
example = parse_events('example.ics')

print(f'Event count: report={len(report)}, example={len(example)}')
print()

event_diffs = 0
for i, (r, e) in enumerate(zip(report, example)):
    diffs = []
    for k in sorted(set(r) | set(e)):
        if k in ('UID', 'DTSTAMP', 'DESCRIPTION'):
            continue
        if r.get(k) != e.get(k):
            diffs.append((k, r.get(k), e.get(k)))
    if diffs:
        event_diffs += 1
        print(f'--- Event {i+1} ---')
        for k, rv, ev in diffs:
            print(f'  {k}:')
            print(f'    REPORT:  {rv}')
            print(f'    EXAMPLE: {ev}')

if not event_diffs:
    print('No per-event differences (excluding UID/DTSTAMP/DESCRIPTION).')
