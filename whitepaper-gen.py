#!/usr/bin/env python3
"""
龙虾卫士 (lobster-guard) 产品白皮书 PDF 生成器
v20.6.0 · 2026-03
"""

from reportlab.lib.pagesizes import A4
from reportlab.lib.units import mm, cm
from reportlab.lib.colors import HexColor, white, black
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.enums import TA_LEFT, TA_CENTER, TA_JUSTIFY, TA_RIGHT
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, PageBreak, Table, TableStyle,
    KeepTogether, HRFlowable, ListFlowable, ListItem
)
from reportlab.pdfgen import canvas
from reportlab.lib.fonts import addMapping
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
import os

# ── Colors ──
INDIGO      = HexColor('#6366F1')
INDIGO_DARK = HexColor('#4F46E5')
INDIGO_LIGHT= HexColor('#A5B4FC')
DARK_BG     = HexColor('#0F172A')
DARK_CARD   = HexColor('#1E293B')
SLATE_700   = HexColor('#334155')
SLATE_500   = HexColor('#64748B')
SLATE_300   = HexColor('#CBD5E1')
SLATE_100   = HexColor('#F1F5F9')
RED_500     = HexColor('#EF4444')
AMBER_500   = HexColor('#F59E0B')
GREEN_500   = HexColor('#22C55E')
CYAN_500    = HexColor('#06B6D4')

# ── Try to register a CJK font ──
CJK_FONT = 'Helvetica'
CJK_FONT_BOLD = 'Helvetica-Bold'
for font_path in [
    '/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc',
    '/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc',
    '/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc',
    '/usr/share/fonts/google-noto-cjk/NotoSansCJKsc-Regular.otf',
    '/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc',
    '/usr/share/fonts/wqy-zenhei/wqy-zenhei.ttc',
    '/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf',
]:
    if os.path.exists(font_path):
        try:
            pdfmetrics.registerFont(TTFont('CJK', font_path, subfontIndex=0))
            CJK_FONT = 'CJK'
            # Try bold variant
            bold_path = font_path.replace('Regular', 'Bold')
            if os.path.exists(bold_path):
                pdfmetrics.registerFont(TTFont('CJK-Bold', bold_path, subfontIndex=0))
                CJK_FONT_BOLD = 'CJK-Bold'
            else:
                CJK_FONT_BOLD = 'CJK'
            print(f"Using CJK font: {font_path}")
            break
        except Exception as e:
            print(f"Failed to load {font_path}: {e}")

if CJK_FONT == 'Helvetica':
    print("WARNING: No CJK font found. Chinese text may not render.")

# ── Styles ──
def make_styles():
    s = {}
    s['cover_title'] = ParagraphStyle(
        'cover_title', fontName=CJK_FONT_BOLD, fontSize=36, leading=44,
        textColor=white, alignment=TA_LEFT, spaceAfter=8*mm
    )
    s['cover_subtitle'] = ParagraphStyle(
        'cover_subtitle', fontName=CJK_FONT, fontSize=16, leading=22,
        textColor=INDIGO_LIGHT, alignment=TA_LEFT, spaceAfter=4*mm
    )
    s['cover_meta'] = ParagraphStyle(
        'cover_meta', fontName=CJK_FONT, fontSize=11, leading=16,
        textColor=SLATE_300, alignment=TA_LEFT
    )
    s['h1'] = ParagraphStyle(
        'h1', fontName=CJK_FONT_BOLD, fontSize=22, leading=28,
        textColor=INDIGO_DARK, spaceBefore=12*mm, spaceAfter=6*mm
    )
    s['h2'] = ParagraphStyle(
        'h2', fontName=CJK_FONT_BOLD, fontSize=16, leading=22,
        textColor=SLATE_700, spaceBefore=8*mm, spaceAfter=4*mm
    )
    s['h3'] = ParagraphStyle(
        'h3', fontName=CJK_FONT_BOLD, fontSize=13, leading=18,
        textColor=SLATE_700, spaceBefore=5*mm, spaceAfter=3*mm
    )
    s['body'] = ParagraphStyle(
        'body', fontName=CJK_FONT, fontSize=10.5, leading=17,
        textColor=SLATE_700, alignment=TA_JUSTIFY, spaceAfter=3*mm
    )
    s['body_bold'] = ParagraphStyle(
        'body_bold', fontName=CJK_FONT_BOLD, fontSize=10.5, leading=17,
        textColor=SLATE_700, alignment=TA_JUSTIFY, spaceAfter=3*mm
    )
    s['bullet'] = ParagraphStyle(
        'bullet', fontName=CJK_FONT, fontSize=10.5, leading=17,
        textColor=SLATE_700, leftIndent=8*mm, bulletIndent=3*mm,
        spaceAfter=1.5*mm
    )
    s['quote'] = ParagraphStyle(
        'quote', fontName=CJK_FONT, fontSize=11, leading=18,
        textColor=INDIGO_DARK, leftIndent=10*mm, rightIndent=10*mm,
        spaceBefore=4*mm, spaceAfter=4*mm, borderPadding=4*mm,
        borderColor=INDIGO_LIGHT, borderWidth=0, alignment=TA_LEFT
    )
    s['caption'] = ParagraphStyle(
        'caption', fontName=CJK_FONT, fontSize=9, leading=13,
        textColor=SLATE_500, alignment=TA_CENTER, spaceAfter=4*mm
    )
    s['toc_item'] = ParagraphStyle(
        'toc_item', fontName=CJK_FONT, fontSize=11, leading=20,
        textColor=SLATE_700, leftIndent=5*mm
    )
    s['toc_h1'] = ParagraphStyle(
        'toc_h1', fontName=CJK_FONT_BOLD, fontSize=12, leading=22,
        textColor=INDIGO_DARK
    )
    s['footer'] = ParagraphStyle(
        'footer', fontName=CJK_FONT, fontSize=8, leading=10,
        textColor=SLATE_500, alignment=TA_CENTER
    )
    s['page_num'] = ParagraphStyle(
        'page_num', fontName=CJK_FONT, fontSize=8, leading=10,
        textColor=SLATE_500, alignment=TA_RIGHT
    )
    return s

ST = make_styles()

# ── Helper functions ──
def colored_hr():
    return HRFlowable(width="100%", thickness=1, color=INDIGO_LIGHT, spaceAfter=4*mm, spaceBefore=2*mm)

def section_spacer():
    return Spacer(1, 6*mm)

def para(text, style='body'):
    return Paragraph(text, ST[style])

def bullet_list(items, style='bullet'):
    return [Paragraph(f"<bullet>&bull;</bullet> {item}", ST[style]) for item in items]

def stat_table(data, col_widths=None):
    """Create a styled data table"""
    if col_widths is None:
        col_widths = [45*mm, 120*mm]
    
    style = TableStyle([
        ('FONTNAME', (0, 0), (-1, 0), CJK_FONT_BOLD),
        ('FONTNAME', (0, 1), (-1, -1), CJK_FONT),
        ('FONTSIZE', (0, 0), (-1, -1), 10),
        ('LEADING', (0, 0), (-1, -1), 15),
        ('TEXTCOLOR', (0, 0), (-1, 0), white),
        ('BACKGROUND', (0, 0), (-1, 0), INDIGO),
        ('TEXTCOLOR', (0, 1), (-1, -1), SLATE_700),
        ('BACKGROUND', (0, 1), (-1, -1), SLATE_100),
        ('ROWBACKGROUNDS', (0, 1), (-1, -1), [white, SLATE_100]),
        ('ALIGN', (0, 0), (-1, -1), 'LEFT'),
        ('VALIGN', (0, 0), (-1, -1), 'MIDDLE'),
        ('TOPPADDING', (0, 0), (-1, -1), 4),
        ('BOTTOMPADDING', (0, 0), (-1, -1), 4),
        ('LEFTPADDING', (0, 0), (-1, -1), 6),
        ('RIGHTPADDING', (0, 0), (-1, -1), 6),
        ('GRID', (0, 0), (-1, -1), 0.5, SLATE_300),
    ])
    
    # Wrap text cells in Paragraphs for long content
    wrapped = []
    for row in data:
        wrapped_row = []
        for cell in row:
            if isinstance(cell, str) and len(cell) > 30:
                wrapped_row.append(Paragraph(cell, ST['body']))
            else:
                wrapped_row.append(cell)
        wrapped.append(wrapped_row)
    
    t = Table(wrapped, colWidths=col_widths, repeatRows=1)
    t.setStyle(style)
    return t

# ── Page template callbacks ──
def cover_page(canvas_obj, doc):
    """Draw the cover page background"""
    c = canvas_obj
    w, h = A4
    
    # Dark background
    c.setFillColor(DARK_BG)
    c.rect(0, 0, w, h, fill=1, stroke=0)
    
    # Indigo accent strip at top
    c.setFillColor(INDIGO)
    c.rect(0, h - 8*mm, w, 8*mm, fill=1, stroke=0)
    
    # Lobster emoji area (large circle)
    c.setFillColor(INDIGO_DARK)
    c.circle(w - 60*mm, 80*mm, 40*mm, fill=1, stroke=0)
    
    # Version badge
    c.setFillColor(INDIGO)
    c.roundRect(25*mm, 25*mm, 50*mm, 8*mm, 3*mm, fill=1, stroke=0)
    c.setFillColor(white)
    c.setFont(CJK_FONT_BOLD, 9)
    c.drawString(30*mm, 27.5*mm, "v20.6.0  |  2026.03")

def later_pages(canvas_obj, doc):
    """Header/footer for content pages"""
    c = canvas_obj
    w, h = A4
    
    # Top line
    c.setStrokeColor(INDIGO_LIGHT)
    c.setLineWidth(0.5)
    c.line(25*mm, h - 15*mm, w - 25*mm, h - 15*mm)
    
    # Header text
    c.setFillColor(SLATE_500)
    c.setFont(CJK_FONT, 7.5)
    c.drawString(25*mm, h - 13*mm, "lobster-guard | AI Agent Security Gateway")
    c.drawRightString(w - 25*mm, h - 13*mm, "Product Whitepaper v20.6.0")
    
    # Bottom line
    c.line(25*mm, 18*mm, w - 25*mm, 18*mm)
    
    # Page number
    c.setFont(CJK_FONT, 8)
    c.drawCentredString(w / 2, 12*mm, str(doc.page))
    
    # Bottom left
    c.setFont(CJK_FONT, 7)
    c.drawString(25*mm, 12*mm, "Confidential")

# ── Build content ──
def build_whitepaper():
    output_path = "/root/.openclaw/workspace-lanxin-2285568-DWuKDtwPA76as2yrjxwsllnGc3VWoO/lobster-guard/lobster-guard-whitepaper-v20.6.pdf"
    
    doc = SimpleDocTemplate(
        output_path,
        pagesize=A4,
        leftMargin=25*mm, rightMargin=25*mm,
        topMargin=22*mm, bottomMargin=25*mm,
        title="lobster-guard Product Whitepaper",
        author="lobster-guard Team",
        subject="AI Agent Security Gateway",
    )
    
    story = []
    
    # ═══════════════════════════════════════════
    # COVER PAGE
    # ═══════════════════════════════════════════
    story.append(Spacer(1, 55*mm))
    story.append(para("lobster-guard", 'cover_title'))
    story.append(para("AI Agent Security Gateway", 'cover_subtitle'))
    story.append(Spacer(1, 8*mm))
    story.append(para("Product Whitepaper", 'cover_meta'))
    story.append(Spacer(1, 4*mm))
    story.append(para("Version 20.6.0  |  March 2026", 'cover_meta'))
    story.append(Spacer(1, 20*mm))
    
    cover_bullets = [
        "Dual Security Domain: IM + LLM",
        "Cryptographic Audit Trail with Merkle Tree",
        "Adversarial Self-Evolution Engine",
        "Taint Propagation Tracking",
        "Single Binary, 4 Dependencies",
    ]
    for b in cover_bullets:
        story.append(Paragraph(
            f'<font color="#A5B4FC"><bullet>&bull;</bullet> {b}</font>',
            ParagraphStyle('cb', fontName=CJK_FONT, fontSize=11, leading=18,
                          textColor=INDIGO_LIGHT, leftIndent=8*mm, bulletIndent=3*mm,
                          spaceAfter=2*mm)
        ))
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════
    # TABLE OF CONTENTS
    # ═══════════════════════════════════════════
    story.append(para("Contents", 'h1'))
    story.append(colored_hr())
    
    toc_items = [
        ("1", "Executive Summary"),
        ("2", "The Problem: AI Agent Security Gap"),
        ("3", "Product Overview"),
        ("4", "Architecture"),
        ("5", "Core Capabilities"),
        ("6", "Dual Security Domain"),
        ("7", "Advanced Defense Mechanisms"),
        ("8", "Dashboard & Operations"),
        ("9", "Deployment"),
        ("10", "Competitive Landscape"),
        ("11", "Use Cases"),
        ("12", "Technical Specifications"),
        ("13", "Roadmap"),
        ("14", "Why lobster-guard"),
    ]
    for num, title in toc_items:
        story.append(Paragraph(
            f'<font color="{INDIGO.hexval()}">{num}.</font>  {title}',
            ST['toc_item']
        ))
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════
    # 1. EXECUTIVE SUMMARY
    # ═══════════════════════════════════════════
    story.append(para("1. Executive Summary", 'h1'))
    story.append(colored_hr())
    
    story.append(para(
        "<b>lobster-guard</b> is a full-featured AI Agent Security Gateway, purpose-built to protect "
        "AI Agents (such as OpenClaw, Dify, Coze, and custom LLM applications) from prompt injection, "
        "data leakage, and adversarial manipulation."
    ))
    story.append(para(
        "Deployed as a transparent reverse proxy between messaging platforms and AI Agents, "
        "lobster-guard inspects every inbound message before it reaches the Agent, audits every "
        "outbound response before it reaches users, and monitors every LLM API call in real-time. "
        "<b>All this happens in a single binary with only 4 external dependencies.</b>"
    ))
    story.append(section_spacer())
    
    # Key metrics box
    metrics = [
        ["Metric", "Value"],
        ["Inbound Detection", "AC Automaton + Regex + Semantic (4-tier pipeline)"],
        ["Outbound Protection", "PII / Credentials / Command injection blocking"],
        ["LLM Audit", "Dual security domain, token cost tracking, tool call policy"],
        ["Detection Rules", "66 built-in templates across 4 industries"],
        ["Platform Support", "LanXin, Feishu, DingTalk, WeCom, Generic HTTP"],
        ["Dashboard", "38 pages, 21 components, Vue 3"],
        ["API Endpoints", "~275 RESTful routes"],
        ["Test Coverage", "950 test cases, all passing"],
        ["External Dependencies", "4 (sqlite3, yaml.v3, gorilla/websocket, x/crypto)"],
        ["Deployment", "Single binary, Docker, Kubernetes"],
    ]
    story.append(stat_table(metrics, col_widths=[42*mm, 118*mm]))
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════
    # 2. THE PROBLEM
    # ═══════════════════════════════════════════
    story.append(para("2. The Problem: AI Agent Security Gap", 'h1'))
    story.append(colored_hr())
    
    story.append(para(
        "Enterprise AI Agent adoption is accelerating. Organizations connect LLM-powered agents to "
        "internal messaging platforms (Feishu, DingTalk, WeCom, LanXin) to handle email triage, "
        "approval workflows, customer service, and knowledge queries. But this creates a new attack surface."
    ))
    
    story.append(para("2.1 Emerging Threat Landscape", 'h2'))
    
    threats = [
        "<b>Prompt Injection</b> -- Attackers embed malicious instructions in normal-looking messages, "
        "hijacking the Agent's behavior to bypass controls, extract data, or execute unauthorized actions.",
        
        "<b>Data Leakage</b> -- Agents with access to internal systems (email, CRM, HR) can inadvertently "
        "expose PII, credentials, API keys, or confidential documents in their responses.",
        
        "<b>Tool Abuse</b> -- LLM-powered Agents make autonomous tool calls (code execution, database queries, "
        "file operations). Without oversight, a compromised Agent becomes an insider threat.",
        
        "<b>Adversarial Evolution</b> -- Attack techniques evolve faster than static rule-based defenses. "
        "Jailbreak prompts mutate daily; yesterday's detection rules miss today's attacks.",
        
        "<b>Invisible Data Flow</b> -- In multi-Agent systems, sensitive data can propagate across Agents "
        "through shared context, creating untraceable information flow pollution.",
        
        "<b>Audit Void</b> -- Most Agent frameworks produce no security audit trail. When a breach occurs, "
        "there is no forensic evidence of what happened, when, or how.",
    ]
    for t in threats:
        story.append(Paragraph(f"<bullet>&bull;</bullet> {t}", ST['bullet']))
    
    story.append(section_spacer())
    story.append(para(
        "Traditional WAFs and API gateways were designed for HTTP request/response patterns. "
        "They do not understand conversational context, multi-turn attacks, LLM tool calls, "
        "or the semantic nature of prompt injection. <b>A new category of security infrastructure is needed.</b>"
    ))
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════
    # 3. PRODUCT OVERVIEW
    # ═══════════════════════════════════════════
    story.append(para("3. Product Overview", 'h1'))
    story.append(colored_hr())
    
    story.append(para(
        "lobster-guard is a transparent security proxy that sits between messaging platforms, "
        "AI Agents, and LLM APIs. It requires <b>zero changes to existing Agent code</b> -- simply "
        "redirect traffic through lobster-guard, and protection is immediate."
    ))
    
    story.append(para("3.1 One-Sentence Definition", 'h2'))
    story.append(Paragraph(
        '<font color="#6366F1"><i>"Every message inspected before it reaches your Agent. '
        'Every response audited before it reaches your users. '
        'Every LLM call monitored in real-time."</i></font>',
        ST['quote']
    ))
    
    story.append(para("3.2 Design Principles", 'h2'))
    principles = [
        "<b>Transparent Proxy</b> -- Zero code changes. Point your traffic at lobster-guard; it handles the rest.",
        "<b>Fail-Open</b> -- Detection failures never block business. Better to miss than to false-positive.",
        "<b>Single Binary</b> -- One Go binary with embedded Dashboard. Copy, configure, run.",
        "<b>Minimal Dependencies</b> -- Only 4 external packages. No Redis, no Kafka, no Kubernetes required.",
        "<b>Backward Compatible</b> -- Every new version works with old configurations. No forced migrations.",
        "<b>Defense in Depth</b> -- 5-layer detection: Pattern Matching, Semantic Analysis, Behavioral Analysis, Cryptographic Assurance, Self-Evolution.",
    ]
    for p in principles:
        story.append(Paragraph(f"<bullet>&bull;</bullet> {p}", ST['bullet']))
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════
    # 4. ARCHITECTURE
    # ═══════════════════════════════════════════
    story.append(para("4. Architecture", 'h1'))
    story.append(colored_hr())
    
    story.append(para("4.1 Four-Port Architecture", 'h2'))
    story.append(para(
        "lobster-guard exposes four dedicated ports, each serving a distinct security domain:"
    ))
    
    port_data = [
        ["Port", "Function", "Security Domain"],
        [":18443", "IM Inbound Proxy", "IM Security Domain"],
        [":18444", "IM Outbound Proxy", "IM Security Domain"],
        [":8445", "LLM Reverse Proxy", "LLM Security Domain"],
        [":9090", "Dashboard + API + Metrics", "Management Layer"],
    ]
    story.append(stat_table(port_data, col_widths=[25*mm, 55*mm, 55*mm]))
    story.append(section_spacer())
    
    story.append(para("4.2 Data Flow", 'h2'))
    story.append(para(
        "<b>Inbound Path:</b> Messaging Platform -> :18443 (decrypt, detect, route) -> AI Agent (OpenClaw)<br/>"
        "<b>Outbound Path:</b> AI Agent -> :18444 (PII scan, credential check, taint verification) -> Messaging Platform API<br/>"
        "<b>LLM Path:</b> Agent -> :8445 (rule engine, tool call policy, caching) -> LLM API (Anthropic/OpenAI)<br/>"
        "<b>Management:</b> Security Team -> :9090 (Dashboard, 275 APIs, Prometheus metrics)"
    ))
    
    story.append(para("4.3 Channel Plugin Architecture", 'h2'))
    story.append(para(
        "Each messaging platform has a dedicated plugin handling its unique encryption, signature verification, "
        "and message format. The security engine operates above this layer, completely platform-agnostic."
    ))
    
    channel_data = [
        ["Platform", "Encryption", "Signature", "Bridge Mode"],
        ["LanXin", "AES-256-CBC", "SHA1", "N/A"],
        ["Feishu", "AES-256-CBC", "SHA256", "WSS"],
        ["DingTalk", "AES-256-CBC", "HMAC-SHA256", "WSS"],
        ["WeCom", "AES-256-CBC", "XML Signature", "N/A"],
        ["Generic HTTP", "Plaintext JSON", "Optional Token", "N/A"],
    ]
    story.append(stat_table(channel_data, col_widths=[28*mm, 35*mm, 35*mm, 30*mm]))
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════
    # 5. CORE CAPABILITIES
    # ═══════════════════════════════════════════
    story.append(para("5. Core Capabilities", 'h1'))
    story.append(colored_hr())
    
    story.append(para("5.1 Multi-Layer Detection Pipeline", 'h2'))
    story.append(para(
        "Inbound messages pass through a configurable detection pipeline. Each layer adds specificity; "
        "early layers are fast (microseconds), later layers are deep (milliseconds)."
    ))
    
    pipeline_data = [
        ["Layer", "Engine", "Speed", "What It Catches"],
        ["L1", "AC Automaton", "< 1 us", "Known prompt injection patterns (40+ rules)"],
        ["L2", "Regex Engine", "< 100 us", "Custom patterns with 100ms timeout protection"],
        ["L3", "Semantic Detector", "< 10 ms", "TF-IDF + syntax + anomaly + intent analysis"],
        ["L4", "Session Detector", "< 1 ms", "Multi-turn attacks, risk score accumulation"],
        ["L5", "LLM Detector", "< 2 s", "Optional deep semantic analysis via external LLM"],
    ]
    story.append(stat_table(pipeline_data, col_widths=[15*mm, 32*mm, 20*mm, 90*mm]))
    story.append(section_spacer())
    
    story.append(para("5.2 Outbound Protection", 'h2'))
    story.append(para(
        "Every response from the AI Agent is scanned before delivery. 6 built-in rules always active, "
        "with user-defined rules merged intelligently (same-name override, new-name append)."
    ))
    
    outbound_data = [
        ["Rule", "Detects", "Action"],
        ["pii_id_card", "National ID numbers", "Block"],
        ["pii_phone", "Phone numbers", "Warn"],
        ["pii_bank_card", "Bank card numbers", "Block"],
        ["credential_password", "Password leakage patterns", "Block"],
        ["credential_apikey", "API keys (sk-/ghp_/AKIA)", "Block"],
        ["malicious_command", "Shell commands (rm -rf, curl|bash)", "Block"],
    ]
    story.append(stat_table(outbound_data, col_widths=[35*mm, 55*mm, 25*mm]))
    
    story.append(para("5.3 LLM Security Domain", 'h2'))
    story.append(para(
        "The LLM reverse proxy (:8445) provides a second security domain specifically for LLM API traffic. "
        "It includes 11 default rules, Canary Token leak detection, Shadow Mode for safe rule testing, "
        "token cost tracking, and tool call policy enforcement."
    ))
    
    story.append(para("5.4 Tool Call Policy Engine", 'h2'))
    story.append(para(
        "When an LLM decides to call tools (execute_code, shell_exec, read_file, database_query), "
        "lobster-guard intercepts the tool_calls in the LLM response and applies policy rules. "
        "18 built-in rules with wildcard matching, parameter keyword detection, and sliding-window rate limiting."
    ))
    
    story.append(PageBreak())
    
    # ═══════════════════════════════════════════
    # 6. DUAL SECURITY DOMAIN
    # ═══════════════════════════════════════════
    story.append(para("6. Dual Security Domain", 'h1'))
    story.append(colored_hr())
    
    story.append(para(
        "lobster-guard is architecturally divided into two independent security domains, each with "
        "its own rule engine, audit log, and management interface."
    ))
    
    domain_data = [
        ["", "IM Security Domain", "LLM Security Domain"],
        ["Ports", ":18443 (inbound) + :18444 (outbound)", ":8445 (LLM proxy)"],
        ["Traffic", "User <-> Agent messages", "Agent <-> LLM API calls"],
        ["Detection", "AC automaton + regex + semantic + session", "LLM rules + Canary + tool policy"],
        ["Audit Table", "audit_log (SQLite)", "llm_audit_log (SQLite)"],
        ["Rules", "Inbound 40+ / Outbound 6+", "LLM 11+ / Tool 18+"],
        ["Special", "Multi-platform, Bridge Mode", "Token cost, response caching"],
    ]
    story.append(stat_table(domain_data, col_widths=[28*mm, 65*mm, 65*mm]))
    story.append(section_spacer())
    
    story.append(para(
        "This separation ensures that IM-side incidents do not interfere with LLM-side operations, "
        "and vice versa. Each domain has independent health checks, independent alerting, "
        "and independent performance metrics."
    ))
    
    story.append(PageBreak())

    # ═══════════════════════════════════════════
    # 7. ADVANCED DEFENSE MECHANISMS
    # ═══════════════════════════════════════════
    story.append(para("7. Advanced Defense Mechanisms", 'h1'))
    story.append(colored_hr())

    story.append(para("7.1 Cryptographic Audit Chain", 'h2'))
    story.append(para(
        "Every security decision generates an <b>Execution Envelope</b> signed with HMAC-SHA256. "
        "Envelopes are batch-verified via Merkle Trees. This transforms audit logs from "
        "'we say we recorded it' to 'mathematics proves we recorded it' -- non-repudiable and tamper-proof."
    ))

    story.append(para("7.2 Adversarial Self-Evolution", 'h2'))
    story.append(para(
        "Inspired by biological immune systems, lobster-guard continuously attacks itself to improve. "
        "The Evolution Engine runs 6 mutation strategies (synonym substitution, encoding transformation, "
        "multilingual translation, context injection, structural morphing, hybrid) on its 33 built-in "
        "attack vectors. When mutations bypass detection, the engine automatically extracts the bypass "
        "pattern and generates new detection rules -- hot-loaded without restart."
    ))
    story.append(para(
        "<i>Theoretical basis: Dissipative structures (Prigogine). Security entropy increases inevitably; "
        "the only defense is continuous energy injection. The Red Team engine is that energy.</i>"
    ))

    story.append(para("7.3 Taint Propagation Tracking", 'h2'))
    story.append(para(
        "When PII is detected in an inbound message, the trace_id is tagged (PII-TAINTED, CONFIDENTIAL, "
        "CREDENTIAL, INTERNAL-ONLY). This tag propagates through the LLM call and outbound response. "
        "Even if the LLM rewrites or summarizes the data beyond regex recognition, the taint tag "
        "triggers blocking at the outbound proxy. <b>We track data lineage, not just content.</b>"
    ))

    story.append(para("7.4 Taint Reversal Engine", 'h2'))
    story.append(para(
        "When taint contamination is detected, the Reversal Engine can inject counter-prompts to "
        "neutralize the contamination. Three modes: <b>soft</b> (append remediation prompt), "
        "<b>hard</b> (replace response entirely), <b>stealth</b> (invisible marker for downstream Agents). "
        "12 built-in reversal templates categorized by contamination type."
    ))

    story.append(para("7.5 Semantic Detection Engine", 'h2'))
    story.append(para(
        "Pure Go implementation using TF-IDF vectorization across four dimensions: "
        "lexical similarity, syntactic structure, statistical anomaly, and intent classification. "
        "47 built-in attack patterns. Zero external ML dependencies -- consistent with the minimal "
        "dependency philosophy."
    ))

    story.append(para("7.6 Agent Honeypots", 'h2'))
    story.append(para(
        "8 pre-built honeypot templates that deploy fake credentials, fake system prompts, "
        "and watermarked data. When an attacker uses the fake information, "
        "the watermark triggers detection, linking back to attacker profiles and session replays. "
        "Loyalty curves track attacker engagement over time."
    ))

    story.append(para("7.7 LLM Response Cache", 'h2'))
    story.append(para(
        "Semantically similar queries hit cache (TF-IDF cosine similarity above configurable threshold), "
        "saving LLM costs and reducing latency. Cache is tenant-isolated and taint-aware -- "
        "contaminated responses are never cached. LRU + TTL eviction with security-event-triggered purge."
    ))

    story.append(PageBreak())

    # ═══════════════════════════════════════════
    # 8. DASHBOARD & OPERATIONS
    # ═══════════════════════════════════════════
    story.append(para("8. Dashboard & Operations Center", 'h1'))
    story.append(colored_hr())

    story.append(para(
        "The management dashboard is a Vue 3 single-page application embedded in the Go binary "
        "via go:embed. Indigo (#6366F1) color scheme inspired by Linear and Vercel."
    ))

    story.append(para("8.1 Key Pages (38 Total)", 'h2'))

    pages_data = [
        ["Category", "Pages", "Highlights"],
        ["Overview", "IM Overview, LLM Overview", "Security health score, OWASP LLM Top 10 matrix"],
        ["Audit", "IM Audit, LLM Audit, Session Replay", "Full-text search, trace_id navigation, timeline"],
        ["Rules", "Inbound Rules, Outbound Rules, LLM Rules", "CRUD, YAML import/export, regex tester"],
        ["Analysis", "Attacker Profile, Behavior Profile, Attack Chain", "Risk scoring, mutation detection, kill chain"],
        ["Defense", "Red Team, Honeypot, A/B Testing", "33 vectors, 8 templates, statistical significance"],
        ["Advanced", "Taint Tracking, Semantic Detection, Tool Policy", "Propagation chain, 4D analysis, 18 rules"],
        ["Infrastructure", "Execution Envelopes, Event Bus, API Gateway", "Merkle verification, webhook, JWT"],
        ["Operations", "Reports, Ops Toolbox, Settings, Leaderboard", "Daily/weekly/monthly, backup, diagnostics"],
        ["Visualization", "Big Screen, Custom Dashboard, Threat Map", "Full-screen SOC wall, drag-and-drop layout"],
    ]
    story.append(stat_table(pages_data, col_widths=[28*mm, 50*mm, 80*mm]))
    story.append(section_spacer())

    story.append(para("8.2 Situational Awareness Big Screen", 'h2'))
    story.append(para(
        "Full-screen immersive mode (F11) designed for SOC walls and conference room displays. "
        "Features include scrolling event stream, auto-rotating panels (OWASP matrix, attack trends, "
        "risk TOP5), core metrics in oversized typography, 4K support. Three display modes: "
        "Classic, Narrative, and Threat Map (ring topology)."
    ))

    story.append(para("8.3 Multi-Tenant & RBAC", 'h2'))
    story.append(para(
        "JWT authentication with three roles (admin / operator / viewer). "
        "Full multi-tenant isolation -- each tenant has independent rules, audit logs, and dashboards. "
        "Tenant switcher in the top bar for cross-tenant management."
    ))

    story.append(PageBreak())

    # ═══════════════════════════════════════════
    # 9. DEPLOYMENT
    # ═══════════════════════════════════════════
    story.append(para("9. Deployment", 'h1'))
    story.append(colored_hr())

    story.append(para("9.1 Deployment Options", 'h2'))

    deploy_data = [
        ["Method", "Best For", "Effort"],
        ["Direct binary", "Quick evaluation, development", "< 5 minutes"],
        ["Systemd service", "Production single-server", "< 15 minutes"],
        ["Docker / Docker Compose", "Containerized environments", "< 10 minutes"],
        ["Kubernetes", "Orchestrated, auto-scaling", "< 30 minutes"],
    ]
    story.append(stat_table(deploy_data, col_widths=[38*mm, 60*mm, 35*mm]))
    story.append(section_spacer())

    story.append(para("9.2 Minimal Configuration", 'h2'))
    story.append(para(
        "Core config.yaml is ~70 lines. Advanced features live in conf.d/ (10 module files). "
        "Don't configure conf.d/? Everything still works -- features degrade gracefully."
    ))

    story.append(para("9.3 Kubernetes Service Discovery", 'h2'))
    story.append(para(
        "When deployed in Kubernetes, lobster-guard automatically discovers OpenClaw pods via the "
        "Kubernetes API (InCluster or Kubeconfig mode). New pods are registered as upstreams with "
        "health checks. Zero external dependencies -- uses Go's native HTTP client, not client-go."
    ))

    story.append(PageBreak())

    # ═══════════════════════════════════════════
    # 10. COMPETITIVE LANDSCAPE
    # ═══════════════════════════════════════════
    story.append(para("10. Competitive Landscape", 'h1'))
    story.append(colored_hr())

    comp_data = [
        ["Product", "Category", "lobster-guard Differentiator"],
        ["Cloudflare AI Gateway", "LLM Gateway", "Dual domain (IM+LLM), single binary, IM channel support"],
        ["Guardrails AI", "Validator Framework", "Transparent proxy (no code change), real-time dashboard"],
        ["Langfuse", "LLM Observability", "Security-first, session replay, attacker profiling"],
        ["MVAR", "Agent Runtime Security", "Gateway-level (no Agent code modification), dual domain"],
        ["Telos", "Kernel-level Agent Security", "Application-layer (cross-platform), enterprise dashboard"],
        ["Kvlar", "MCP Policy Engine", "Complete security operations platform, not just policies"],
        ["OWASP LLM Top 10", "Risk Framework", "Every risk mapped to concrete detection rules + metrics"],
    ]
    story.append(stat_table(comp_data, col_widths=[35*mm, 38*mm, 85*mm]))
    story.append(section_spacer())

    story.append(para(
        "<b>Unique positioning:</b> lobster-guard is the only product that combines IM-channel security "
        "(multi-platform encryption/decryption), LLM API security (tool call policy, response caching), "
        "and a full security operations center (38-page dashboard, Red Team, behavior analysis) "
        "in a single binary with 4 dependencies."
    ))

    story.append(PageBreak())

    # ═══════════════════════════════════════════
    # 11. USE CASES
    # ═══════════════════════════════════════════
    story.append(para("11. Use Cases", 'h1'))
    story.append(colored_hr())

    story.append(para("11.1 Enterprise IM Agent Protection", 'h2'))
    story.append(para(
        "A company deploys an AI Agent on Feishu/DingTalk to handle employee queries (email lookup, "
        "leave requests, expense approvals). Without lobster-guard, a single prompt injection message "
        "could instruct the Agent to forward all unread emails to the attacker. "
        "With lobster-guard: the injection is blocked at L1/L2, the attacker is profiled, "
        "the security team receives a webhook alert -- all in <b>under 50 milliseconds</b>."
    ))

    story.append(para("11.2 LLM API Cost & Security Governance", 'h2'))
    story.append(para(
        "A development team uses GPT-4/Claude for code generation. Without governance, "
        "a single runaway Agent can burn through $10,000 in API costs overnight. "
        "lobster-guard's LLM proxy provides per-tenant token tracking, budget alerts (Canary Token), "
        "response caching (reducing repeat queries by 30-50%), and tool call restrictions."
    ))

    story.append(para("11.3 Compliance & Audit", 'h2'))
    story.append(para(
        "Regulated industries (finance, healthcare, government) require provable audit trails. "
        "lobster-guard's cryptographic Execution Envelopes provide mathematical proof that every "
        "security decision was recorded, timestamped, and signed. Merkle Tree batches enable "
        "efficient bulk verification. Built-in report templates (daily/weekly/monthly) with "
        "HTML inline CSS for email-friendly distribution."
    ))

    story.append(para("11.4 Security Operations Center (SOC)", 'h2'))
    story.append(para(
        "Security teams monitoring multiple AI Agents across the organization. "
        "The situational awareness big screen displays real-time attack trends, OWASP LLM Top 10 "
        "matrix coverage, security health scores, and anomaly alerts. "
        "Multi-tenant support allows one lobster-guard instance to serve the entire enterprise."
    ))

    story.append(PageBreak())

    # ═══════════════════════════════════════════
    # 12. TECHNICAL SPECIFICATIONS
    # ═══════════════════════════════════════════
    story.append(para("12. Technical Specifications", 'h1'))
    story.append(colored_hr())

    spec_data = [
        ["Specification", "Detail"],
        ["Language", "Go 1.21+ (backend), Vue 3.5 + Vite 6.2 (frontend)"],
        ["Binary Size", "Single executable with embedded dashboard (go:embed)"],
        ["Database", "SQLite with WAL mode (zero external database required)"],
        ["External Dependencies", "4: go-sqlite3, yaml.v3, gorilla/websocket, x/crypto"],
        ["Source Code", "70 source files + 50 test files = ~71,100 lines of Go"],
        ["Frontend", "38 pages + 21 components = 65 Vue files, ~20,400 lines"],
        ["API", "~275 RESTful endpoints"],
        ["Tests", "950 test functions, all passing"],
        ["Authentication", "JWT (HS256, self-implemented) + Bearer Token"],
        ["Encryption", "AES-256-CBC (channel), HMAC-SHA256 (envelopes), bcrypt (passwords)"],
        ["Detection Speed", "L1: < 1 us, L2: < 100 us, L3: < 10 ms, L4: < 50 ms"],
        ["Platforms", "Linux (production), macOS (development)"],
        ["Container", "Multi-stage Dockerfile, Docker Compose, K8s manifests"],
        ["Configuration", "YAML, ~70 lines core + conf.d/ modular (10 files)"],
        ["License", "MIT"],
    ]
    story.append(stat_table(spec_data, col_widths=[40*mm, 118*mm]))

    story.append(PageBreak())

    # ═══════════════════════════════════════════
    # 13. ROADMAP
    # ═══════════════════════════════════════════
    story.append(para("13. Roadmap", 'h1'))
    story.append(colored_hr())

    story.append(para(
        "lobster-guard follows a phased development strategy. Phase 1 (v18-v20, completed) maximizes "
        "value from existing data flows without requiring upstream changes. Phase 2 (v21+) introduces "
        "architectural evolution requiring protocol coordination."
    ))

    road_data = [
        ["Phase", "Version", "Focus"],
        ["Completed", "v1-v17", "Core proxy, multi-channel, detection, dashboard, behavior analysis"],
        ["Phase 1 (Done)", "v18-v20", "Cryptographic audit, self-evolution, semantic detection, taint tracking, caching, API gateway"],
        ["Phase 2 (Planned)", "v21-v22", "Agent identity protocol, MCP Proxy, cross-Agent worm detection"],
        ["Phase 3 (Future)", "v23-v24", "AI security copilot, guardrail marketplace, distributed deployment"],
    ]
    story.append(stat_table(road_data, col_widths=[30*mm, 28*mm, 100*mm]))
    story.append(section_spacer())

    story.append(para("Each major version is grounded in theoretical foundations:", 'body_bold'))
    theory_items = [
        "v18 -- Goedel's Incompleteness: Security cannot self-prove; cryptography approximates provability",
        "v19 -- Entropy & Dissipative Structures: Security degrades inevitably; continuous red-team energy counters entropy",
        "v20 -- Shannon Information Theory: trace_id as taint carrier; three-segment tracking",
        "v21 -- Halting Problem / Rice's Theorem: Perfect detection is impossible; whitelisting > blacklisting",
        "v22 -- Emergent security: Single-Agent safety does not guarantee multi-Agent safety",
    ]
    for item in theory_items:
        story.append(Paragraph(f"<bullet>&bull;</bullet> {item}", ST['bullet']))

    story.append(PageBreak())

    # ═══════════════════════════════════════════
    # 14. WHY LOBSTER-GUARD
    # ═══════════════════════════════════════════
    story.append(para("14. Why lobster-guard", 'h1'))
    story.append(colored_hr())

    story.append(para(
        "In a world where AI Agents are becoming the primary interface between humans and enterprise systems, "
        "security cannot be an afterthought. lobster-guard was built from day one with a single mission: "
        "<b>make AI Agent deployment safe without making it slow, complex, or invasive.</b>"
    ))
    story.append(section_spacer())

    why_items = [
        "<b>Zero Invasion</b> -- Your Agent code stays untouched. No SDK, no wrapper, no middleware injection. "
        "lobster-guard is infrastructure, not a library.",

        "<b>Defense in Depth</b> -- Five detection layers from microsecond pattern matching to "
        "behavioral analysis. Cryptographic proof that every decision was recorded.",

        "<b>Self-Healing</b> -- The Evolution Engine ensures detection rules improve automatically. "
        "Yesterday's bypass becomes today's detection rule.",

        "<b>Operational Simplicity</b> -- Single binary, 4 dependencies, 70-line config. "
        "From download to protection in under 5 minutes.",

        "<b>Enterprise Ready</b> -- Multi-tenant, JWT RBAC, compliance reports, SOC big screen, "
        "Kubernetes native. Built for teams, not just individuals.",

        "<b>Open Source</b> -- MIT licensed. Full source code on GitHub. "
        "No vendor lock-in, no phone-home, no telemetry.",
    ]
    for item in why_items:
        story.append(Paragraph(f"<bullet>&bull;</bullet> {item}", ST['bullet']))

    story.append(Spacer(1, 15*mm))
    story.append(Paragraph(
        '<font color="#6366F1" size="14"><b>'
        'Protecting your upstream, one lobster at a time.'
        '</b></font>',
        ParagraphStyle('closing', fontName=CJK_FONT_BOLD, fontSize=14, leading=20,
                      alignment=TA_CENTER, textColor=INDIGO, spaceAfter=8*mm)
    ))

    story.append(Spacer(1, 10*mm))
    story.append(colored_hr())
    story.append(para("GitHub: https://github.com/zhuowater/lobster-guard", 'caption'))
    story.append(para("License: MIT  |  Version: 20.6.0  |  March 2026", 'caption'))

    # ── Build ──
    doc.build(story, onFirstPage=cover_page, onLaterPages=later_pages)
    print(f"\n{'='*60}")
    print(f"Whitepaper generated: {output_path}")
    print(f"{'='*60}")
    return output_path

if __name__ == "__main__":
    build_whitepaper()