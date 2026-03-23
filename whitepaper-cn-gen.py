#!/usr/bin/env python3
"""
龙虾卫士 (lobster-guard) 产品白皮书 中文版 + 销售一纸禅
v20.6.0 · 2026-03
"""

from reportlab.lib.pagesizes import A4
from reportlab.lib.units import mm, cm
from reportlab.lib.colors import HexColor, white, black
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.enums import TA_LEFT, TA_CENTER, TA_JUSTIFY, TA_RIGHT
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, PageBreak, Table, TableStyle,
    KeepTogether, HRFlowable
)
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
import os

# ── Colors ──
INDIGO       = HexColor('#6366F1')
INDIGO_DARK  = HexColor('#4F46E5')
INDIGO_LIGHT = HexColor('#A5B4FC')
DARK_BG      = HexColor('#0F172A')
SLATE_700    = HexColor('#334155')
SLATE_500    = HexColor('#64748B')
SLATE_300    = HexColor('#CBD5E1')
SLATE_100    = HexColor('#F1F5F9')
RED_500      = HexColor('#EF4444')
GREEN_500    = HexColor('#22C55E')
AMBER_500    = HexColor('#F59E0B')

# ── CJK Font ──
CJK = 'Helvetica'
CJKB = 'Helvetica-Bold'
for fp in [
    '/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc',
    '/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc',
    '/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc',
    '/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc',
]:
    if os.path.exists(fp):
        try:
            pdfmetrics.registerFont(TTFont('CJK', fp, subfontIndex=0))
            CJK = 'CJK'
            bp = fp.replace('Regular', 'Bold').replace('zenhei', 'zenhei')
            if os.path.exists(bp):
                pdfmetrics.registerFont(TTFont('CJKB', bp, subfontIndex=0))
                CJKB = 'CJKB'
            else:
                CJKB = 'CJK'
            break
        except:
            pass

# ── Styles ──
def S(name, **kw):
    defaults = dict(fontName=CJK, fontSize=10.5, leading=17, textColor=SLATE_700, spaceAfter=3*mm)
    defaults.update(kw)
    return ParagraphStyle(name, **defaults)

ST = {
    'cover_title': S('ct', fontName=CJKB, fontSize=38, leading=48, textColor=white, spaceAfter=8*mm),
    'cover_sub':   S('cs', fontSize=16, leading=22, textColor=INDIGO_LIGHT, spaceAfter=4*mm),
    'cover_meta':  S('cm', fontSize=11, leading=16, textColor=SLATE_300),
    'h1':    S('h1', fontName=CJKB, fontSize=22, leading=28, textColor=INDIGO_DARK, spaceBefore=10*mm, spaceAfter=5*mm),
    'h2':    S('h2', fontName=CJKB, fontSize=15, leading=21, textColor=SLATE_700, spaceBefore=7*mm, spaceAfter=3.5*mm),
    'h3':    S('h3', fontName=CJKB, fontSize=12.5, leading=18, textColor=SLATE_700, spaceBefore=5*mm, spaceAfter=2.5*mm),
    'body':  S('body', alignment=TA_JUSTIFY),
    'bodyb': S('bodyb', fontName=CJKB, alignment=TA_JUSTIFY),
    'bullet':S('bul', leftIndent=8*mm, bulletIndent=3*mm, spaceAfter=1.5*mm),
    'quote': S('quote', fontSize=11.5, leading=19, textColor=INDIGO_DARK, leftIndent=8*mm, rightIndent=8*mm, spaceBefore=4*mm, spaceAfter=4*mm),
    'caption':S('cap', fontSize=9, leading=13, textColor=SLATE_500, alignment=TA_CENTER, spaceAfter=4*mm),
    'toc':   S('toc', fontSize=11, leading=22),
    'tocb':  S('tocb', fontName=CJKB, fontSize=12, leading=24, textColor=INDIGO_DARK),
}

def hr():
    return HRFlowable(width="100%", thickness=1, color=INDIGO_LIGHT, spaceAfter=4*mm, spaceBefore=2*mm)

def p(text, style='body'):
    return Paragraph(text, ST[style])

def bl(items):
    return [Paragraph(f"<bullet>&bull;</bullet> {i}", ST['bullet']) for i in items]

def tbl(data, widths=None):
    if not widths:
        widths = [40*mm] + [120*mm]
    wrapped = []
    for row in data:
        wr = []
        for cell in row:
            if isinstance(cell, str) and len(cell) > 25:
                wr.append(Paragraph(cell, S('tc', fontSize=9.5, leading=14, textColor=SLATE_700, spaceAfter=0)))
            else:
                wr.append(cell)
        wrapped.append(wr)
    t = Table(wrapped, colWidths=widths, repeatRows=1)
    t.setStyle(TableStyle([
        ('FONTNAME', (0,0), (-1,0), CJKB),
        ('FONTNAME', (0,1), (-1,-1), CJK),
        ('FONTSIZE', (0,0), (-1,-1), 9.5),
        ('LEADING', (0,0), (-1,-1), 14),
        ('TEXTCOLOR', (0,0), (-1,0), white),
        ('BACKGROUND', (0,0), (-1,0), INDIGO),
        ('TEXTCOLOR', (0,1), (-1,-1), SLATE_700),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [white, SLATE_100]),
        ('ALIGN', (0,0), (-1,-1), 'LEFT'),
        ('VALIGN', (0,0), (-1,-1), 'MIDDLE'),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('LEFTPADDING', (0,0), (-1,-1), 5),
        ('RIGHTPADDING', (0,0), (-1,-1), 5),
        ('GRID', (0,0), (-1,-1), 0.5, SLATE_300),
    ]))
    return t

# ── Page decorators ──
def cover_page(c, doc):
    w, h = A4
    c.setFillColor(DARK_BG); c.rect(0, 0, w, h, fill=1, stroke=0)
    c.setFillColor(INDIGO); c.rect(0, h-8*mm, w, 8*mm, fill=1, stroke=0)
    c.setFillColor(INDIGO_DARK); c.circle(w-60*mm, 80*mm, 40*mm, fill=1, stroke=0)
    c.setFillColor(INDIGO); c.roundRect(25*mm, 25*mm, 55*mm, 8*mm, 3*mm, fill=1, stroke=0)
    c.setFillColor(white); c.setFont(CJKB, 9); c.drawString(30*mm, 27.5*mm, "v20.6.0  |  2026.03")

def later_pages(c, doc):
    w, h = A4
    c.setStrokeColor(INDIGO_LIGHT); c.setLineWidth(0.5)
    c.line(25*mm, h-15*mm, w-25*mm, h-15*mm)
    c.setFillColor(SLATE_500); c.setFont(CJK, 7.5)
    c.drawString(25*mm, h-13*mm, "lobster-guard | AI Agent Security Gateway")
    c.drawRightString(w-25*mm, h-13*mm, "Product Whitepaper v20.6.0")
    c.line(25*mm, 18*mm, w-25*mm, 18*mm)
    c.setFont(CJK, 8); c.drawCentredString(w/2, 12*mm, str(doc.page))

# ═══════════════════════════════════════════════════════════
#  PART 1: 中文白皮书
# ═══════════════════════════════════════════════════════════
def build_whitepaper_cn():
    path = "/root/.openclaw/workspace-lanxin-2285568-DWuKDtwPA76as2yrjxwsllnGc3VWoO/lobster-guard/lobster-guard-whitepaper-v20.6-cn.pdf"
    doc = SimpleDocTemplate(path, pagesize=A4, leftMargin=25*mm, rightMargin=25*mm,
                           topMargin=22*mm, bottomMargin=25*mm,
                           title="lobster-guard 产品白皮书", author="lobster-guard")
    story = []

    # ── 封面 ──
    story.append(Spacer(1, 55*mm))
    story.append(p("lobster-guard", 'cover_title'))
    story.append(p("AI Agent 安全网关", 'cover_sub'))
    story.append(Spacer(1, 6*mm))
    story.append(p("产品白皮书", 'cover_meta'))
    story.append(Spacer(1, 3*mm))
    story.append(p("版本 20.6.0  |  2026 年 3 月", 'cover_meta'))
    story.append(Spacer(1, 18*mm))
    for b in [
        "双安全域：IM 通道 + LLM 通道",
        "密码学审计链 (HMAC-SHA256 + Merkle Tree)",
        "对抗性自进化引擎",
        "信息流污染追踪与逆转",
        "单二进制部署，仅 4 个依赖",
    ]:
        story.append(Paragraph(
            f'<font color="#A5B4FC"><bullet>&bull;</bullet> {b}</font>',
            S('cb', fontSize=11, leading=18, textColor=INDIGO_LIGHT, leftIndent=8*mm, bulletIndent=3*mm, spaceAfter=2*mm)
        ))
    story.append(PageBreak())

    # ── 目录 ──
    story.append(p("目录", 'h1')); story.append(hr())
    for n, t in [
        ("1","执行摘要"), ("2","问题：AI Agent 安全缺口"), ("3","产品概述"),
        ("4","系统架构"), ("5","核心能力"), ("6","双安全域"),
        ("7","高级防御机制"), ("8","管理后台与运营中心"), ("9","部署方式"),
        ("10","竞品对比"), ("11","应用场景"), ("12","技术规格"),
        ("13","产品路线图"), ("14","为什么选择龙虾卫士"),
    ]:
        story.append(Paragraph(f'<font color="{INDIGO.hexval()}">{n}.</font>  {t}', ST['toc']))
    story.append(PageBreak())

    # ── 1. 执行摘要 ──
    story.append(p("1. 执行摘要", 'h1')); story.append(hr())
    story.append(p(
        "<b>lobster-guard（龙虾卫士）</b>是一款全功能 AI Agent 安全网关，专为保护 AI Agent"
        "（如 OpenClaw、Dify、Coze 及自研 LLM 应用）免受提示注入、数据泄露和对抗性操纵而设计。"
    ))
    story.append(p(
        "部署为透明反向代理，龙虾卫士在消息到达 Agent 之前拦截检测，在 Agent 响应到达用户之前审计过滤，"
        "在 LLM API 调用全程实时监控。<b>所有这些能力封装在一个二进制文件中，仅依赖 4 个外部包。</b>"
    ))
    story.append(Spacer(1, 4*mm))
    story.append(tbl([
        ["指标", "数值"],
        ["入站检测", "AC 自动机 + 正则 + 语义分析（4 层流水线）"],
        ["出站防护", "PII / 凭据 / 恶意命令拦截"],
        ["LLM 审计", "双安全域、Token 成本追踪、工具调用策略"],
        ["检测规则", "66 条内置模板，覆盖 4 个行业"],
        ["平台支持", "蓝信、飞书、钉钉、企业微信、通用 HTTP"],
        ["管理后台", "38 个页面、21 个组件、Vue 3"],
        ["API 接口", "约 275 个 RESTful 路由"],
        ["测试覆盖", "950 个测试用例，全部通过"],
        ["外部依赖", "4 个（sqlite3 + yaml.v3 + websocket + x/crypto）"],
        ["部署方式", "单二进制 / Docker / Kubernetes"],
    ], [38*mm, 122*mm]))
    story.append(PageBreak())

    # ── 2. 问题 ──
    story.append(p("2. 问题：AI Agent 安全缺口", 'h1')); story.append(hr())
    story.append(p(
        "企业正在加速部署 AI Agent。组织将 LLM 驱动的智能体接入内部 IM 平台（飞书、钉钉、企业微信、蓝信），"
        "用于处理邮件分拣、审批流程、客户服务和知识查询。但这创造了一个全新的攻击面。"
    ))
    story.append(p("2.1 新兴威胁图景", 'h2'))
    story.extend(bl([
        "<b>提示注入（Prompt Injection）</b>—— 攻击者在看似正常的消息中嵌入恶意指令，劫持 Agent 行为，"
        "绕过访问控制、窃取数据或执行未授权操作。",
        "<b>数据泄露</b>—— 具有内部系统访问权限的 Agent 可能在响应中暴露个人身份信息、凭据、API 密钥或机密文档。",
        "<b>工具滥用</b>—— LLM 驱动的 Agent 可自主调用工具（代码执行、数据库查询、文件操作）。"
        "失控的 Agent 等同于内部威胁。",
        "<b>对抗性进化</b>—— 攻击技术的进化速度远超静态规则的更新速度。昨天的检测规则漏掉今天的攻击。",
        "<b>隐形数据流</b>—— 在多 Agent 系统中，敏感数据通过共享上下文在 Agent 之间传播，形成不可追踪的信息流污染。",
        "<b>审计真空</b>—— 大多数 Agent 框架不产生安全审计轨迹。当泄露发生时，没有取证证据。",
    ]))
    story.append(Spacer(1, 4*mm))
    story.append(p(
        "传统 WAF 和 API 网关针对 HTTP 请求/响应模式设计，无法理解对话上下文、多轮攻击、LLM 工具调用"
        "和提示注入的语义本质。<b>需要一个全新品类的安全基础设施。</b>"
    ))
    story.append(PageBreak())

    # ── 3. 产品概述 ──
    story.append(p("3. 产品概述", 'h1')); story.append(hr())
    story.append(p(
        "龙虾卫士是透明安全代理，部署在消息平台、AI Agent 和 LLM API 之间。"
        "<b>零代码侵入</b>—— 只需将流量指向龙虾卫士，防护即刻生效。"
    ))
    story.append(p("3.1 一句话定义", 'h2'))
    story.append(Paragraph(
        '<font color="#6366F1"><i>"每条消息在到达 Agent 前接受安检。'
        '每条响应在到达用户前接受审计。每次 LLM 调用实时监控。"</i></font>', ST['quote']
    ))
    story.append(p("3.2 设计原则", 'h2'))
    story.extend(bl([
        "<b>透明代理</b>—— 零代码修改。将流量指向龙虾卫士，其余交给它。",
        "<b>Fail-Open</b>—— 检测异常不阻塞业务。宁可漏检不可误杀。",
        "<b>单二进制</b>—— 一个 Go 二进制文件内嵌 Dashboard。复制、配置、运行。",
        "<b>极简依赖</b>—— 仅 4 个外部包。不需要 Redis、Kafka 或 Kubernetes。",
        "<b>向后兼容</b>—— 新版本始终兼容旧配置。无强制迁移。",
        "<b>纵深防御</b>—— 5 层检测：模式匹配 > 语义分析 > 行为分析 > 密码学保证 > 自进化。",
    ]))
    story.append(PageBreak())

    # ── 4. 架构 ──
    story.append(p("4. 系统架构", 'h1')); story.append(hr())
    story.append(p("4.1 四端口架构", 'h2'))
    story.append(tbl([
        ["端口", "功能", "安全域"],
        [":18443", "IM 入站代理", "IM 安全域"],
        [":18444", "IM 出站代理", "IM 安全域"],
        [":8445", "LLM 反向代理", "LLM 安全域"],
        [":9090", "Dashboard + API + Metrics", "管理层"],
    ], [22*mm, 55*mm, 45*mm]))
    story.append(Spacer(1, 5*mm))

    story.append(p("4.2 数据流", 'h2'))
    story.append(p(
        "<b>入站路径：</b>消息平台 -> :18443（解密、检测、路由）-> AI Agent<br/>"
        "<b>出站路径：</b>AI Agent -> :18444（PII 扫描、凭据检查、污染验证）-> 消息平台 API<br/>"
        "<b>LLM 路径：</b>Agent -> :8445（规则引擎、工具策略、缓存）-> LLM API<br/>"
        "<b>管理路径：</b>安全团队 -> :9090（Dashboard、275 API、Prometheus 指标）"
    ))

    story.append(p("4.3 通道插件架构", 'h2'))
    story.append(tbl([
        ["平台", "加密方式", "签名", "Bridge Mode"],
        ["蓝信", "AES-256-CBC", "SHA1", "—"],
        ["飞书", "AES-256-CBC", "SHA256", "WSS 长连接"],
        ["钉钉", "AES-256-CBC", "HMAC-SHA256", "WSS 长连接"],
        ["企业微信", "AES-256-CBC", "XML 签名", "—"],
        ["通用 HTTP", "明文 JSON", "可选 Token", "—"],
    ], [25*mm, 35*mm, 32*mm, 30*mm]))
    story.append(PageBreak())

    # ── 5. 核心能力 ──
    story.append(p("5. 核心能力", 'h1')); story.append(hr())
    story.append(p("5.1 多层检测流水线", 'h2'))
    story.append(tbl([
        ["层级", "引擎", "速度", "检测内容"],
        ["L1", "AC 自动机", "< 1 us", "已知提示注入模式（40+ 规则）"],
        ["L2", "正则引擎", "< 100 us", "自定义模式，100ms 超时保护"],
        ["L3", "语义检测器", "< 10 ms", "TF-IDF + 句法 + 异常 + 意图四维分析"],
        ["L4", "会话检测器", "< 1 ms", "多轮攻击识别，风险积分累加"],
        ["L5", "LLM 检测器", "< 2 s", "可选深度语义分析（外部 LLM）"],
    ], [15*mm, 28*mm, 20*mm, 90*mm]))
    story.append(Spacer(1, 5*mm))

    story.append(p("5.2 出站防护", 'h2'))
    story.append(tbl([
        ["规则", "检测内容", "动作"],
        ["pii_id_card", "身份证号码", "拦截"],
        ["pii_phone", "手机号码", "告警"],
        ["pii_bank_card", "银行卡号", "拦截"],
        ["credential_password", "密码泄露模式", "拦截"],
        ["credential_apikey", "API 密钥（sk-/ghp_/AKIA）", "拦截"],
        ["malicious_command", "Shell 命令（rm -rf, curl|bash）", "拦截"],
    ], [35*mm, 55*mm, 20*mm]))
    story.append(Spacer(1, 5*mm))

    story.append(p("5.3 LLM 安全域", 'h2'))
    story.append(p(
        "LLM 反向代理（:8445）提供独立的第二安全域。包含 11 条默认规则、Canary Token 泄露检测、"
        "Shadow Mode 安全测试模式、Token 成本追踪、工具调用策略执行。"
    ))

    story.append(p("5.4 工具调用策略引擎", 'h2'))
    story.append(p(
        "当 LLM 决定调用工具（execute_code、shell_exec、read_file、database_query）时，"
        "龙虾卫士拦截 tool_calls 并应用策略规则。18 条内置规则，支持通配符匹配、参数关键词检测和滑动窗口限流。"
    ))
    story.append(PageBreak())

    # ── 6. 双安全域 ──
    story.append(p("6. 双安全域架构", 'h1')); story.append(hr())
    story.append(tbl([
        ["维度", "IM 安全域", "LLM 安全域"],
        ["端口", ":18443（入站）+ :18444（出站）", ":8445（LLM 代理）"],
        ["流量", "用户 <-> Agent 消息", "Agent <-> LLM API 调用"],
        ["检测", "AC 自动机 + 正则 + 语义 + 会话", "LLM 规则 + Canary + 工具策略"],
        ["审计表", "audit_log（SQLite）", "llm_audit_log（SQLite）"],
        ["规则数", "入站 40+ / 出站 6+", "LLM 11+ / 工具 18+"],
        ["特殊能力", "多平台、Bridge Mode", "Token 成本、响应缓存"],
    ], [22*mm, 68*mm, 68*mm]))
    story.append(Spacer(1, 5*mm))
    story.append(p(
        "两个安全域完全独立运作——IM 侧事件不影响 LLM 侧运行，反之亦然。"
        "各自有独立的健康检查、告警通道和性能指标。"
    ))
    story.append(PageBreak())

    # ── 7. 高级防御 ──
    story.append(p("7. 高级防御机制", 'h1')); story.append(hr())

    story.append(p("7.1 密码学审计链", 'h2'))
    story.append(p(
        "每个安全决策生成 <b>执行信封（Execution Envelope）</b>，使用 HMAC-SHA256 签名，"
        "通过 Merkle Tree 批量验证。审计日志从「我说我记了」变成「数学证明我记了」——不可否认、不可篡改。"
    ))

    story.append(p("7.2 对抗性自进化引擎", 'h2'))
    story.append(p(
        "受生物免疫系统启发，龙虾卫士持续攻击自身以提升防御。进化引擎运行 6 种变异策略"
        "（同义替换、编码变形、多语言翻译、上下文注入、结构变异、混合），对 33 个内置攻击向量进行变异。"
        "当变异体绕过检测时，引擎自动提取绕过模式并生成新检测规则——热加载，无需重启。"
    ))
    story.append(p(
        "<i>理论基础：耗散结构（Prigogine）。安全熵增不可避免；唯一的防御是持续注入能量。红队引擎就是那股能量。</i>"
    ))

    story.append(p("7.3 信息流污染追踪", 'h2'))
    story.append(p(
        "入站检测到 PII 时，trace_id 被标记（PII-TAINTED / CONFIDENTIAL / CREDENTIAL / INTERNAL-ONLY）。"
        "标签随 LLM 调用和出站响应传播。即使 LLM 改写或摘要了数据导致正则无法匹配原始 PII，"
        "污染标签仍触发出站拦截。<b>我们追踪数据血统，而不仅是内容。</b>"
    ))

    story.append(p("7.4 污染逆转引擎", 'h2'))
    story.append(p(
        "检测到污染后，逆转引擎注入反向提示词中和污染。三种模式：<b>soft</b>（追加修复提示）、"
        "<b>hard</b>（完全替换响应）、<b>stealth</b>（不可见标记供下游 Agent 识别）。"
        "12 个内置逆转模板，按污染类型分类。"
    ))

    story.append(p("7.5 语义检测引擎", 'h2'))
    story.append(p(
        "纯 Go 实现，使用 TF-IDF 向量化进行四维分析：词汇相似度、句法结构、统计异常、意图分类。"
        "47 个内置攻击模式。零外部 ML 依赖——与极简依赖哲学一致。"
    ))

    story.append(p("7.6 Agent 蜜罐", 'h2'))
    story.append(p(
        "8 个预置蜜罐模板，部署假凭据、假系统提示和水印数据。攻击者使用假信息时，"
        "水印触发检测，关联攻击者画像和会话回放。忠诚度曲线追踪攻击者参与度变化。"
    ))

    story.append(p("7.7 LLM 响应缓存", 'h2'))
    story.append(p(
        "语义相似查询命中缓存（TF-IDF 余弦相似度超过阈值），节省 LLM 成本和延迟。"
        "缓存按租户隔离、污染感知——被污染的响应不进缓存。LRU + TTL 淘汰，安全事件触发清除。"
    ))
    story.append(PageBreak())

    # ── 8. 管理后台 ──
    story.append(p("8. 管理后台与运营中心", 'h1')); story.append(hr())
    story.append(p(
        "管理后台是嵌入 Go 二进制的 Vue 3 单页应用（go:embed）。"
        "Indigo (#6366F1) 配色方案，参考 Linear 和 Vercel 设计语言。"
    ))
    story.append(tbl([
        ["类别", "页面", "亮点"],
        ["概览", "IM 概览、LLM 概览", "安全健康分、OWASP LLM Top 10 矩阵"],
        ["审计", "IM 审计、LLM 审计、会话回放", "全文搜索、trace_id 导航、时间线"],
        ["规则", "入站规则、出站规则、LLM 规则", "CRUD、YAML 导入导出、正则测试器"],
        ["分析", "攻击者画像、行为画像、攻击链", "风险评分、突变检测、Kill Chain"],
        ["防御", "红队、蜜罐、A/B 测试", "33 向量、8 模板、统计显著性"],
        ["高级", "污染追踪、语义检测、工具策略", "传播链、四维分析、18 规则"],
        ["基础设施", "执行信封、事件总线、API 网关", "Merkle 验证、Webhook、JWT"],
        ["运维", "报告、运维工具箱、设置、排行榜", "日报/周报/月报、备份、诊断"],
        ["可视化", "态势大屏、自定义仪表盘、威胁地图", "全屏 SOC 墙、拖拽布局"],
    ], [24*mm, 55*mm, 78*mm]))
    story.append(Spacer(1, 5*mm))
    story.append(p(
        "<b>态势感知大屏：</b>全屏沉浸式模式（F11），为 SOC 大墙和会议室投影设计。"
        "支持滚动事件流、自动轮播面板、超大字体核心指标、4K 适配。三种显示模式：经典、叙事、威胁地图（环形拓扑）。"
    ))
    story.append(p(
        "<b>多租户与角色控制：</b>JWT 认证，三个角色（admin / operator / viewer）。"
        "完全租户隔离——每个租户有独立的规则、审计日志和仪表盘。"
    ))
    story.append(PageBreak())

    # ── 9. 部署 ──
    story.append(p("9. 部署方式", 'h1')); story.append(hr())
    story.append(tbl([
        ["方式", "适用场景", "上手时间"],
        ["直接运行二进制", "快速评估、开发环境", "< 5 分钟"],
        ["Systemd 服务", "生产环境单机部署", "< 15 分钟"],
        ["Docker / Docker Compose", "容器化环境", "< 10 分钟"],
        ["Kubernetes", "编排、自动扩缩", "< 30 分钟"],
    ], [38*mm, 60*mm, 30*mm]))
    story.append(Spacer(1, 5*mm))
    story.append(p(
        "<b>极简配置：</b>核心 config.yaml 仅约 70 行。高级功能拆分到 conf.d/ 目录（10 个模块文件）。"
        "不配 conf.d/ 也能正常运行——功能优雅降级。"
    ))
    story.append(p(
        "<b>Kubernetes 服务发现：</b>在 K8s 环境中，龙虾卫士通过 Kubernetes API 自动发现 OpenClaw Pod，"
        "自动注册为上游并开启健康检查。零外部依赖——使用 Go 原生 HTTP 客户端，非 client-go。"
    ))
    story.append(PageBreak())

    # ── 10. 竞品 ──
    story.append(p("10. 竞品对比", 'h1')); story.append(hr())
    story.append(tbl([
        ["产品", "品类", "龙虾卫士差异化"],
        ["Cloudflare AI Gateway", "LLM 网关", "双安全域（IM+LLM）、单二进制、IM 通道支持"],
        ["Guardrails AI", "验证器框架", "透明代理（零代码）、实时 Dashboard"],
        ["Langfuse", "LLM 可观测性", "安全优先、会话回放、攻击者画像"],
        ["MVAR", "Agent 运行时安全", "网关级（不改 Agent 代码）、双安全域"],
        ["Telos", "内核级 Agent 安全", "应用层实现（跨平台）、企业级 Dashboard"],
        ["Kvlar", "MCP 策略引擎", "完整安全运营平台，非仅策略引擎"],
        ["OWASP LLM Top 10", "风险框架", "每项风险对应具体检测规则和指标"],
    ], [35*mm, 32*mm, 90*mm]))
    story.append(Spacer(1, 5*mm))
    story.append(p(
        "<b>独特定位：</b>龙虾卫士是唯一将 IM 通道安全（多平台加解密）、LLM API 安全（工具策略、响应缓存）"
        "和完整安全运营中心（38 页面 Dashboard、红队、行为分析）集成在单一二进制中且仅 4 个依赖的产品。"
    ))
    story.append(PageBreak())

    # ── 11. 场景 ──
    story.append(p("11. 应用场景", 'h1')); story.append(hr())
    story.append(p("11.1 企业 IM Agent 安全防护", 'h2'))
    story.append(p(
        "企业在飞书/钉钉部署 AI Agent 处理员工查询（邮件查找、请假审批、报销审核）。"
        "没有龙虾卫士时，一条提示注入消息即可指示 Agent 将所有未读邮件转发给攻击者。"
        "有龙虾卫士：注入在 L1/L2 被拦截，攻击者被画像标记，安全团队收到 Webhook 告警——"
        "全程 <b>50 毫秒以内</b>。"
    ))
    story.append(p("11.2 LLM API 成本与安全治理", 'h2'))
    story.append(p(
        "开发团队使用 GPT-4/Claude 进行代码生成。缺乏治理时，一个失控 Agent 可在一夜间烧掉数万元 API 费用。"
        "龙虾卫士 LLM 代理提供按租户 Token 追踪、预算告警（Canary Token）、"
        "响应缓存（降低重复查询 30-50%）和工具调用限制。"
    ))
    story.append(p("11.3 合规与审计", 'h2'))
    story.append(p(
        "金融、医疗、政务等监管行业要求可证明的审计轨迹。龙虾卫士的密码学执行信封提供数学证明——"
        "每个安全决策均已记录、时间戳标注和签名。Merkle Tree 批次支持高效批量验证。"
        "内置报告模板（日报/周报/月报），HTML 内联 CSS 便于邮件发送。"
    ))
    story.append(p("11.4 安全运营中心（SOC）", 'h2'))
    story.append(p(
        "安全团队监控全组织多个 AI Agent。态势感知大屏实时展示攻击趋势、OWASP LLM Top 10 矩阵覆盖率、"
        "安全健康评分和异常告警。多租户支持允许单个龙虾卫士实例服务整个企业。"
    ))
    story.append(PageBreak())

    # ── 12. 技术规格 ──
    story.append(p("12. 技术规格", 'h1')); story.append(hr())
    story.append(tbl([
        ["规格", "详情"],
        ["开发语言", "Go 1.21+（后端）、Vue 3.5 + Vite 6.2（前端）"],
        ["部署形态", "单可执行文件，内嵌 Dashboard（go:embed）"],
        ["数据库", "SQLite WAL 模式（零外部数据库依赖）"],
        ["外部依赖", "4 个：go-sqlite3、yaml.v3、gorilla/websocket、x/crypto"],
        ["后端代码", "70 个源文件 + 50 个测试文件 = 约 71,100 行 Go"],
        ["前端代码", "38 页面 + 21 组件 = 65 个 Vue 文件，约 20,400 行"],
        ["API 接口", "约 275 个 RESTful 端点"],
        ["测试", "950 个测试函数，全部通过"],
        ["认证", "JWT（HS256 自实现）+ Bearer Token"],
        ["加密", "AES-256-CBC（通道）、HMAC-SHA256（信封）、bcrypt（密码）"],
        ["检测速度", "L1: < 1 us, L2: < 100 us, L3: < 10 ms, L4: < 50 ms"],
        ["运行平台", "Linux（生产）、macOS（开发）"],
        ["容器支持", "多阶段 Dockerfile、Docker Compose、K8s 清单"],
        ["配置", "YAML，约 70 行核心 + conf.d/ 模块化（10 个文件）"],
        ["开源许可", "MIT"],
    ], [30*mm, 128*mm]))
    story.append(PageBreak())

    # ── 13. 路线图 ──
    story.append(p("13. 产品路线图", 'h1')); story.append(hr())
    story.append(tbl([
        ["阶段", "版本", "重点"],
        ["已完成", "v1-v17", "核心代理、多通道、检测、Dashboard、行为分析"],
        ["Phase 1（已完成）", "v18-v20", "密码学审计、自进化、语义检测、污染追踪、缓存、API 网关"],
        ["Phase 2（规划中）", "v21-v22", "Agent 身份协议、MCP Proxy、跨 Agent 蠕虫检测"],
        ["Phase 3（远期）", "v23-v24", "AI 安全副驾驶、Guardrail 市场、分布式部署"],
    ], [30*mm, 25*mm, 103*mm]))
    story.append(Spacer(1, 5*mm))
    story.append(p("每个大版本有理论根基：", 'bodyb'))
    story.extend(bl([
        "v18 — Goedel 不完备定理：安全无法自证，密码学逼近可证明性",
        "v19 — 熵增定律 + 耗散结构：安全退化不可避免，持续红队能量对抗熵增",
        "v20 — Shannon 信息论：trace_id 作为污染载体，三段联合追踪",
        "v21 — 停机问题 / Rice 定理：完美检测不可能，白名单优于黑名单",
        "v22 — 涌现安全：单 Agent 安全不等于多 Agent 安全",
    ]))
    story.append(PageBreak())

    # ── 14. 为什么 ──
    story.append(p("14. 为什么选择龙虾卫士", 'h1')); story.append(hr())
    story.extend(bl([
        "<b>零侵入</b>—— Agent 代码不动。无 SDK、无包装器、无中间件注入。龙虾卫士是基础设施，不是库。",
        "<b>纵深防御</b>—— 五层检测，从微秒级模式匹配到行为分析。密码学证明每个决策均已记录。",
        "<b>自我愈合</b>—— 进化引擎确保检测规则自动改进。昨天的绕过变成今天的检测规则。",
        "<b>运维极简</b>—— 单二进制、4 个依赖、70 行配置。从下载到防护 5 分钟以内。",
        "<b>企业就绪</b>—— 多租户、JWT RBAC、合规报告、SOC 大屏、K8s 原生。为团队而建。",
        "<b>开源开放</b>—— MIT 许可。完整源码在 GitHub。无厂商锁定、无回传、无遥测。",
    ]))
    story.append(Spacer(1, 12*mm))
    story.append(Paragraph(
        '<font color="#6366F1" size="14"><b>保护你的上游，一只龙虾的事。</b></font>',
        S('closing', fontName=CJKB, fontSize=14, leading=20, alignment=TA_CENTER, textColor=INDIGO, spaceAfter=8*mm)
    ))
    story.append(Spacer(1, 8*mm)); story.append(hr())
    story.append(p("GitHub: https://github.com/zhuowater/lobster-guard", 'caption'))
    story.append(p("License: MIT  |  Version: 20.6.0  |  2026 年 3 月", 'caption'))

    doc.build(story, onFirstPage=cover_page, onLaterPages=later_pages)
    print(f"[CN Whitepaper] {path}")
    return path


# ═══════════════════════════════════════════════════════════
#  PART 2: 销售一纸禅 (One-Pager)
# ═══════════════════════════════════════════════════════════
def build_one_pager():
    path = "/root/.openclaw/workspace-lanxin-2285568-DWuKDtwPA76as2yrjxwsllnGc3VWoO/lobster-guard/lobster-guard-one-pager.pdf"
    doc = SimpleDocTemplate(path, pagesize=A4, leftMargin=15*mm, rightMargin=15*mm,
                           topMargin=15*mm, bottomMargin=12*mm)
    story = []

    # ── Title bar ──
    title_data = [[
        Paragraph('<font color="#FFFFFF" size="20"><b>lobster-guard</b></font>', S('t', fontSize=20, textColor=white, spaceAfter=0)),
        Paragraph('<font color="#A5B4FC" size="10">AI Agent 安全网关  |  v20.6.0</font>', S('t2', fontSize=10, textColor=INDIGO_LIGHT, spaceAfter=0, alignment=TA_RIGHT)),
    ]]
    title_tbl = Table(title_data, colWidths=[90*mm, 90*mm])
    title_tbl.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), DARK_BG),
        ('VALIGN', (0,0), (-1,-1), 'MIDDLE'),
        ('TOPPADDING', (0,0), (-1,-1), 8),
        ('BOTTOMPADDING', (0,0), (-1,-1), 8),
        ('LEFTPADDING', (0,0), (0,0), 12),
        ('RIGHTPADDING', (-1,-1), (-1,-1), 12),
        ('ROUNDEDCORNERS', [4, 4, 4, 4]),
    ]))
    story.append(title_tbl)
    story.append(Spacer(1, 4*mm))

    # ── Tagline ──
    story.append(Paragraph(
        '<font color="#6366F1"><b>"每条消息到达 Agent 前安检。每条响应到达用户前审计。每次 LLM 调用实时监控。"</b></font>',
        S('tag', fontName=CJKB, fontSize=10, leading=15, textColor=INDIGO, alignment=TA_CENTER, spaceAfter=4*mm)
    ))

    # ── 问题 & 方案 (两栏) ──
    left_title = Paragraph('<font color="#EF4444"><b>痛点</b></font>', S('lt', fontName=CJKB, fontSize=11, textColor=RED_500, spaceAfter=0))
    right_title = Paragraph('<font color="#22C55E"><b>龙虾卫士方案</b></font>', S('rt', fontName=CJKB, fontSize=11, textColor=GREEN_500, spaceAfter=0))

    def mini(text):
        return Paragraph(text, S('mini', fontSize=8.5, leading=12.5, textColor=SLATE_700, spaceAfter=0))

    pain_items = [
        mini("<bullet>&bull;</bullet> 提示注入劫持 Agent 行为"),
        mini("<bullet>&bull;</bullet> Agent 响应泄露 PII/凭据"),
        mini("<bullet>&bull;</bullet> LLM 工具调用失控（执行代码/删库）"),
        mini("<bullet>&bull;</bullet> 攻击手法日新月异，规则跟不上"),
        mini("<bullet>&bull;</bullet> 多 Agent 间敏感数据隐形扩散"),
        mini("<bullet>&bull;</bullet> 零审计轨迹，出事无法取证"),
    ]
    sol_items = [
        mini("<bullet>&bull;</bullet> 5 层检测（AC 自动机 -> 语义 -> 行为 -> 密码学 -> 自进化）"),
        mini("<bullet>&bull;</bullet> 出站 PII/凭据/命令实时拦截（6+ 规则）"),
        mini("<bullet>&bull;</bullet> 工具调用策略引擎（18 规则 + 限流）"),
        mini("<bullet>&bull;</bullet> 对抗性自进化：自动变异+自动生成新规则"),
        mini("<bullet>&bull;</bullet> 信息流污染追踪（trace_id 全链路标记）"),
        mini("<bullet>&bull;</bullet> 密码学执行信封 + Merkle Tree 审计链"),
    ]

    left_col = [left_title] + pain_items
    right_col = [right_title] + sol_items

    lr_data = [[left_col[i] if i < len(left_col) else '', right_col[i] if i < len(right_col) else ''] for i in range(max(len(left_col), len(right_col)))]
    lr_tbl = Table(lr_data, colWidths=[88*mm, 88*mm])
    lr_tbl.setStyle(TableStyle([
        ('VALIGN', (0,0), (-1,-1), 'TOP'),
        ('TOPPADDING', (0,0), (-1,-1), 2),
        ('BOTTOMPADDING', (0,0), (-1,-1), 2),
        ('LEFTPADDING', (0,0), (-1,-1), 4),
        ('RIGHTPADDING', (0,0), (-1,-1), 4),
        ('LINEAFTER', (0,0), (0,-1), 0.5, SLATE_300),
    ]))
    story.append(lr_tbl)
    story.append(Spacer(1, 3*mm))

    # ── 核心数据 (stat cards) ──
    def stat_card(num, label, color=INDIGO):
        return [
            Paragraph(f'<font color="{color.hexval()}" size="18"><b>{num}</b></font>',
                     S('sn', fontSize=18, alignment=TA_CENTER, textColor=color, spaceAfter=0)),
            Paragraph(f'<font size="7.5">{label}</font>',
                     S('sl', fontSize=7.5, alignment=TA_CENTER, textColor=SLATE_500, spaceAfter=0)),
        ]

    cards = [
        stat_card("5 层", "检测深度"),
        stat_card("< 1us", "L1 检测速度"),
        stat_card("950", "测试用例"),
        stat_card("275", "API 端点"),
        stat_card("38", "Dashboard 页面"),
        stat_card("4", "外部依赖"),
    ]
    card_row = [[cards[i][0] for i in range(6)], [cards[i][1] for i in range(6)]]
    card_tbl = Table(card_row, colWidths=[30*mm]*6)
    card_tbl.setStyle(TableStyle([
        ('ALIGN', (0,0), (-1,-1), 'CENTER'),
        ('VALIGN', (0,0), (-1,-1), 'MIDDLE'),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 2),
        ('BACKGROUND', (0,0), (-1,-1), SLATE_100),
        ('BOX', (0,0), (-1,-1), 0.5, SLATE_300),
        ('INNERGRID', (0,0), (-1,-1), 0.5, SLATE_300),
        ('ROUNDEDCORNERS', [3, 3, 3, 3]),
    ]))
    story.append(card_tbl)
    story.append(Spacer(1, 4*mm))

    # ── 架构图 (文字版) ──
    story.append(Paragraph('<font color="#4F46E5"><b>架构概览</b></font>',
                          S('arch', fontName=CJKB, fontSize=11, textColor=INDIGO_DARK, spaceAfter=2*mm)))

    arch_data = [
        [mini('<font color="#6366F1"><b>消息平台</b></font>\n蓝信/飞书/钉钉/企微'),
         mini('<font color="#EF4444"><b>:18443 入站检测</b></font>\n解密 > 5层检测 > 路由'),
         mini('<font color="#22C55E"><b>AI Agent</b></font>\n(OpenClaw/Dify/Coze)')],
        [mini('<font color="#22C55E"><b>AI Agent</b></font>\n响应/API 调用'),
         mini('<font color="#EF4444"><b>:18444 出站审计</b></font>\nPII > 凭据 > 污染检查'),
         mini('<font color="#6366F1"><b>消息平台 API</b></font>\n安全送达')],
        [mini('<font color="#22C55E"><b>Agent/应用</b></font>\nLLM API 请求'),
         mini('<font color="#F59E0B"><b>:8445 LLM 代理</b></font>\n规则 > 工具策略 > 缓存'),
         mini('<font color="#6366F1"><b>LLM API</b></font>\nAnthropic/OpenAI')],
    ]
    arch_tbl = Table(arch_data, colWidths=[55*mm, 66*mm, 55*mm])
    arch_tbl.setStyle(TableStyle([
        ('ALIGN', (0,0), (-1,-1), 'CENTER'),
        ('VALIGN', (0,0), (-1,-1), 'MIDDLE'),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('BACKGROUND', (0,0), (-1,-1), white),
        ('BOX', (0,0), (-1,-1), 1, INDIGO_LIGHT),
        ('INNERGRID', (0,0), (-1,-1), 0.5, SLATE_300),
        ('ROUNDEDCORNERS', [3, 3, 3, 3]),
    ]))
    story.append(arch_tbl)
    story.append(Spacer(1, 3*mm))

    # ── 应用场景 ──
    story.append(Paragraph('<font color="#4F46E5"><b>典型场景</b></font>',
                          S('sc', fontName=CJKB, fontSize=11, textColor=INDIGO_DARK, spaceAfter=2*mm)))

    scene_data = [
        ["场景", "价值"],
        ["企业 IM Agent 防护", "拦截提示注入 < 50ms，零代码侵入接入"],
        ["LLM 成本治理", "响应缓存节省 30-50%，按租户 Token 追踪"],
        ["合规审计", "密码学执行信封 + Merkle Tree，数学证明审计完整性"],
        ["SOC 安全运营", "38 页面 Dashboard + 态势大屏 + 多租户"],
    ]
    scene_tbl = Table(
        [[Paragraph(c, S('sc2', fontSize=8.5, leading=12, textColor=SLATE_700 if i > 0 else white, fontName=CJKB if i == 0 else CJK, spaceAfter=0))
          for c in row] for i, row in enumerate(scene_data)],
        colWidths=[45*mm, 131*mm]
    )
    scene_tbl.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), INDIGO),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [white, SLATE_100]),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
        ('LEFTPADDING', (0,0), (-1,-1), 5),
        ('GRID', (0,0), (-1,-1), 0.5, SLATE_300),
    ]))
    story.append(scene_tbl)
    story.append(Spacer(1, 3*mm))

    # ── 竞品优势 ──
    story.append(Paragraph('<font color="#4F46E5"><b>差异化优势</b></font>',
                          S('adv', fontName=CJKB, fontSize=11, textColor=INDIGO_DARK, spaceAfter=2*mm)))

    adv_data = [
        [mini("<b>vs Cloudflare AI GW</b>\n双安全域 + IM 通道"),
         mini("<b>vs Guardrails AI</b>\n透明代理无需改代码"),
         mini("<b>vs Langfuse</b>\n安全优先 + 攻击者画像")],
        [mini("<b>vs MVAR/Telos</b>\n网关级部署 + 企业 Dashboard"),
         mini("<b>vs Kvlar</b>\n完整安全运营平台"),
         mini("<b>唯一组合</b>\nIM + LLM + SOC + 单二进制")],
    ]
    adv_tbl = Table(adv_data, colWidths=[59*mm]*3)
    adv_tbl.setStyle(TableStyle([
        ('ALIGN', (0,0), (-1,-1), 'CENTER'),
        ('VALIGN', (0,0), (-1,-1), 'TOP'),
        ('BACKGROUND', (0,0), (-1,-1), SLATE_100),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('INNERGRID', (0,0), (-1,-1), 0.5, SLATE_300),
        ('BOX', (0,0), (-1,-1), 0.5, SLATE_300),
    ]))
    story.append(adv_tbl)
    story.append(Spacer(1, 4*mm))

    # ── 底部 CTA ──
    cta_data = [[
        Paragraph('<font color="#FFFFFF"><b>GitHub</b>  github.com/zhuowater/lobster-guard</font>',
                 S('cta1', fontSize=9, textColor=white, spaceAfter=0)),
        Paragraph('<font color="#A5B4FC">MIT License  |  Single Binary  |  5 Min Setup</font>',
                 S('cta2', fontSize=9, textColor=INDIGO_LIGHT, alignment=TA_RIGHT, spaceAfter=0)),
    ]]
    cta_tbl = Table(cta_data, colWidths=[100*mm, 80*mm])
    cta_tbl.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), DARK_BG),
        ('VALIGN', (0,0), (-1,-1), 'MIDDLE'),
        ('TOPPADDING', (0,0), (-1,-1), 6),
        ('BOTTOMPADDING', (0,0), (-1,-1), 6),
        ('LEFTPADDING', (0,0), (0,0), 10),
        ('RIGHTPADDING', (-1,-1), (-1,-1), 10),
        ('ROUNDEDCORNERS', [3, 3, 3, 3]),
    ]))
    story.append(cta_tbl)

    doc.build(story)
    print(f"[One-Pager] {path}")
    return path


# ── Main ──
if __name__ == "__main__":
    p1 = build_whitepaper_cn()
    p2 = build_one_pager()
    print(f"\nDone! Generated:\n  1. {p1}\n  2. {p2}")