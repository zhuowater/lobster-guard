#!/usr/bin/env python3
import json, time, subprocess, sys, math
from pathlib import Path
from collections import Counter, defaultdict

BASE = Path(__file__).resolve().parent
GT = BASE / 'ground_truth.jsonl'
RESULTS = BASE / 'results.jsonl'
FP = BASE / 'false_positives.jsonl'
FN = BASE / 'false_negatives.jsonl'
REPORT = BASE / 'baseline-report.md'

SSH_HOST = '10.44.96.142'
SSH_PORT = '22022'
SSH_PASS = '055971yu'
TOKEN = 'test-token-2026!'
URL = 'http://localhost:9090/api/v1/detect/test'
MAX_RETRIES = 3
TIMEOUT = 20


def load_jsonl(path):
    rows = []
    with open(path, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            if line:
                rows.append(json.loads(line))
    return rows


_SSH_BASE = [
    'sshpass', '-p', SSH_PASS,
    'ssh', '-o', 'StrictHostKeyChecking=no', '-o', 'ConnectTimeout=8',
    '-o', 'ServerAliveInterval=15', '-o', 'ServerAliveCountMax=3',
    '-o', 'ControlMaster=auto', '-o', 'ControlPersist=600',
    '-o', 'ControlPath=/tmp/lobster-bench-%r@%h:%p',
    '-p', SSH_PORT, f'root@{SSH_HOST}'
]


def detect_remote(text):
    payload = json.dumps({'text': text}, ensure_ascii=False)
    remote_cmd = [
        'curl', '-sS', '-X', 'POST', URL,
        '-H', 'Content-Type: application/json',
        '-H', f'Authorization: Bearer {TOKEN}',
        '--data-binary', payload,
    ]
    last_err = None
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            p = subprocess.run(_SSH_BASE + [subprocess.list2cmdline(remote_cmd)], capture_output=True, text=True, timeout=TIMEOUT)
            if p.returncode == 0 and p.stdout.strip():
                try:
                    return json.loads(p.stdout.strip())
                except Exception as e:
                    last_err = f'json decode failed: {e}; raw={p.stdout[:300]}'
            else:
                last_err = (p.stderr or p.stdout or f'returncode={p.returncode}')[:400]
        except Exception as e:
            last_err = str(e)
        time.sleep(min(1.5 * attempt, 3))
    return {'action': 'error', 'error': last_err}


def pred_label(action):
    act = str(action or '').lower()
    if act in ('block', 'warn'):
        return 'malicious'
    if act in ('pass', 'allow'):
        return 'benign'
    return 'benign'


def safe_div(a, b):
    return a / b if b else 0.0


def compute_metrics(rows):
    tp = sum(1 for r in rows if r['label'] == 'malicious' and r['predicted_label'] == 'malicious')
    fp = sum(1 for r in rows if r['label'] == 'benign' and r['predicted_label'] == 'malicious')
    tn = sum(1 for r in rows if r['label'] == 'benign' and r['predicted_label'] == 'benign')
    fn = sum(1 for r in rows if r['label'] == 'malicious' and r['predicted_label'] == 'benign')
    precision = safe_div(tp, tp + fp)
    recall = safe_div(tp, tp + fn)
    f1 = safe_div(2 * precision * recall, precision + recall)
    return {
        'tp': tp, 'fp': fp, 'tn': tn, 'fn': fn,
        'precision': precision, 'recall': recall, 'f1': f1
    }


def render_report(rows, metrics):
    by_cat = defaultdict(list)
    by_diff = defaultdict(list)
    by_source = defaultdict(list)
    action_counter = Counter(r.get('action', 'unknown') for r in rows)
    for r in rows:
        by_cat[r['category']].append(r)
        by_diff[r['difficulty']].append(r)
        by_source[r['source']].append(r)

    lines = []
    lines.append('# Lobster Guard Baseline Report')
    lines.append('')
    lines.append(f'- Samples: {len(rows)}')
    lines.append(f"- Action distribution: {dict(action_counter)}")
    lines.append('')
    lines.append('## Overall Metrics')
    lines.append('')
    lines.append(f"- Precision: {metrics['precision']:.4f}")
    lines.append(f"- Recall: {metrics['recall']:.4f}")
    lines.append(f"- F1: {metrics['f1']:.4f}")
    lines.append('')
    lines.append('## Confusion Matrix')
    lines.append('')
    lines.append('| Actual \\ Pred | Malicious | Benign |')
    lines.append('|---|---:|---:|')
    lines.append(f"| Malicious | {metrics['tp']} | {metrics['fn']} |")
    lines.append(f"| Benign | {metrics['fp']} | {metrics['tn']} |")
    lines.append('')
    lines.append('## Per-Category Metrics')
    lines.append('')
    lines.append('| Category | Count | Precision | Recall | F1 |')
    lines.append('|---|---:|---:|---:|---:|')
    for cat in ['prompt_injection', 'data_exfil', 'harmful', 'normal']:
        m = compute_metrics(by_cat.get(cat, []))
        lines.append(f"| {cat} | {len(by_cat.get(cat, []))} | {m['precision']:.4f} | {m['recall']:.4f} | {m['f1']:.4f} |")
    lines.append('')
    lines.append('## By Difficulty')
    lines.append('')
    lines.append('| Difficulty | Count | Precision | Recall | F1 |')
    lines.append('|---|---:|---:|---:|---:|')
    for k in ['easy', 'medium', 'hard']:
        m = compute_metrics(by_diff.get(k, []))
        lines.append(f"| {k} | {len(by_diff.get(k, []))} | {m['precision']:.4f} | {m['recall']:.4f} | {m['f1']:.4f} |")
    lines.append('')
    lines.append('## By Source')
    lines.append('')
    lines.append('| Source | Count | Precision | Recall | F1 |')
    lines.append('|---|---:|---:|---:|---:|')
    for k in sorted(by_source):
        m = compute_metrics(by_source[k])
        lines.append(f"| {k} | {len(by_source[k])} | {m['precision']:.4f} | {m['recall']:.4f} | {m['f1']:.4f} |")
    lines.append('')
    lines.append('## Error Analysis')
    lines.append('')
    fps = [r for r in rows if r['label'] == 'benign' and r['predicted_label'] == 'malicious']
    fns = [r for r in rows if r['label'] == 'malicious' and r['predicted_label'] == 'benign']
    lines.append(f'- False positives: {len(fps)}')
    lines.append(f'- False negatives: {len(fns)}')
    if fps:
        lines.append('- Top FP examples:')
        for r in fps[:5]:
            lines.append(f"  - {r['text'][:120]}")
    if fns:
        lines.append('- Top FN examples:')
        for r in fns[:5]:
            lines.append(f"  - {r['text'][:120]}")
    return '\n'.join(lines) + '\n'


def main():
    rows = load_jsonl(GT)
    out = []
    with open(RESULTS, 'w', encoding='utf-8') as rf:
        for i, row in enumerate(rows, 1):
            resp = detect_remote(row['text'])
            action = resp.get('action') or resp.get('result') or resp.get('decision') or 'unknown'
            pred = pred_label(action)
            item = dict(row)
            item.update({
                'index': i,
                'action': action,
                'predicted_label': pred,
                'response': resp,
            })
            rf.write(json.dumps(item, ensure_ascii=False) + '\n')
            out.append(item)
            if i % 25 == 0:
                print(f'processed {i}/{len(rows)}', file=sys.stderr)
    fps = [r for r in out if r['label'] == 'benign' and r['predicted_label'] == 'malicious']
    fns = [r for r in out if r['label'] == 'malicious' and r['predicted_label'] == 'benign']
    with open(FP, 'w', encoding='utf-8') as f:
        for r in fps:
            f.write(json.dumps(r, ensure_ascii=False) + '\n')
    with open(FN, 'w', encoding='utf-8') as f:
        for r in fns:
            f.write(json.dumps(r, ensure_ascii=False) + '\n')
    metrics = compute_metrics(out)
    REPORT.write_text(render_report(out, metrics), encoding='utf-8')
    print(json.dumps(metrics, ensure_ascii=False))

if __name__ == '__main__':
    main()
