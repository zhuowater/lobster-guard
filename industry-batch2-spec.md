# 第二批行业模板规格 (27个)

## 修改文件
- `/tmp/lobster-guard/src/config.go` — getDefaultInboundTemplates()
- `/tmp/lobster-guard/src/llm_rules.go` — getDefaultLLMTemplates()
- `/tmp/lobster-guard/src/path_policy.go` — defaultTemplates

## 同时需要做的
- 将现有"金融行业"(tpl-*-financial)改名为"银行/支付"

## ID 命名规范
入站: tpl-inbound-{slug}
LLM: tpl-llm-{slug}
PathPolicy: tpl-{slug}
LLM规则: tpl-{slug}-req-001, tpl-{slug}-resp-001

## ====== 金融细分 ======

### 13. 证券/投行 (securities)
- 入站: 研报草稿(warn)、IPO材料(block)、持仓明细(warn)
- LLM-req: 查询未公开研报(warn)、导出IPO定价(block)
- LLM-resp: 研报未公开内容泄露(warn)、持仓数据泄露(warn,regex: 持仓.*股|shares.*position)
- PathPolicy: tpl-securities → pp-004,pp-005,pp-011,pp-012
- patterns-cn: 研报草稿,未公开研报,IPO定价,配售方案,持仓明细,交易策略,投行项目,保荐材料,路演材料,锁定期
- patterns-en: draft research,unpublished report,IPO pricing,share allocation,position detail,trading strategy,underwriting,roadshow material

### 14. 基金/资管 (fund)
- 入站: 净值预测(warn)、投资组合(warn)、风控模型(block)
- LLM-req: 查询基金持仓(warn)、导出风控参数(block)
- LLM-resp: 投资组合泄露(warn)、净值预测数据泄露(warn,regex: NAV|净值.*预测|alpha.*模型)
- PathPolicy: tpl-fund → pp-004,pp-005,pp-011,pp-012
- patterns-cn: 净值预测,基金持仓,投资组合,资产配置,回撤数据,夏普比率,风控模型,清盘线,预警线,风险敞口
- patterns-en: NAV forecast,fund holding,portfolio allocation,asset allocation,drawdown,Sharpe ratio,risk model,liquidation line,risk exposure

### 15. 制药/生物科技 (pharma)
- 入站: 药物配方(block)、临床试验数据(warn)、GMP记录(warn)
- LLM-req: 查询药物分子式(block)、导出临床数据(warn)
- LLM-resp: 药物配方泄露(block)、临床试验结果泄露(warn,regex: Phase [I-IV]|期临床|CRO|受试者)
- PathPolicy: tpl-pharma → pp-004,pp-005,pp-009,pp-011,pp-012
- patterns-cn: 药物配方,分子式,合成路线,临床试验,受试者数据,GMP记录,批生产记录,药品注册,IND申请,生物等效性,原料药工艺
- patterns-en: drug formula,molecular structure,synthesis route,clinical trial,subject data,GMP record,batch record,drug registration,IND filing,bioequivalence,API process

## ====== 制造细分 ======

### 16. 机器人/自动化 (robotics)
- 入站: 运动控制算法(block)、安全区域配置(warn)、传感器融合参数(warn)
- LLM-req: 查询控制算法(block)、修改安全区域(block)
- LLM-resp: 控制算法泄露(block)、机器人参数泄露(warn,regex: PID.*参数|轨迹规划|joint.*torque)
- PathPolicy: tpl-robotics → pp-002,pp-005,pp-007,pp-008
- patterns-cn: 运动控制算法,逆运动学,轨迹规划,PID参数,伺服参数,安全区域,协作区域,力控参数,传感器融合,SLAM算法,视觉引导
- patterns-en: motion control algorithm,inverse kinematics,trajectory planning,PID parameter,servo parameter,safety zone,collaborative zone,force control,sensor fusion,SLAM algorithm,visual guidance

### 17. 消费电子/家电 (consumer-electronics)
- 入站: 产品BOM(warn)、模具图纸(block)、供应商报价(warn)
- LLM-req: 导出BOM清单(warn)、查询供应商报价(warn)
- LLM-resp: BOM成本泄露(warn)、模具参数泄露(warn,regex: BOM.*cost|模具.*参数|供应商.*价格)
- PathPolicy: tpl-consumer-electronics → pp-003,pp-004,pp-011
- patterns-cn: 产品BOM,物料清单,模具图纸,模具参数,供应商报价,成本结构,认证数据,3C认证,CE认证,FCC认证,开模费用
- patterns-en: product BOM,bill of materials,mold drawing,mold parameter,supplier quotation,cost structure,certification data,tooling cost

### 18. 重工/装备制造 (heavy-industry)
- 入站: 焊接工艺(warn)、压力容器参数(block)、特种设备数据(warn)
- LLM-req: 查询设备参数(warn)、导出工艺规程(block)
- LLM-resp: 工艺参数泄露(warn)、特种设备数据泄露(warn,regex: 焊接.*工艺|WPS|压力.*容器|特种设备)
- PathPolicy: tpl-heavy-industry → pp-004,pp-005,pp-011
- patterns-cn: 焊接工艺,WPS,焊接规程,压力容器,特种设备,起重机参数,锅炉参数,管道设计,热处理工艺,无损检测,探伤报告
- patterns-en: welding procedure,WPS,welding specification,pressure vessel,special equipment,crane parameter,boiler parameter,piping design,heat treatment,NDT,inspection report

## ====== 交通细分 ======

### 19. 民航 (civil-aviation)
- 入站: 适航数据(block)、飞控参数(block)、旅客PNR(warn)
- LLM-req: 查询飞控配置(block)、导出旅客数据(warn)
- LLM-resp: 飞控参数泄露(block)、航线运营数据泄露(warn,regex: FMS|NOTAM|ACARS|MEL)
- PathPolicy: tpl-civil-aviation → pp-002,pp-005,pp-007,pp-008,pp-011
- patterns-cn: 适航证,飞控参数,FMS配置,航路点,NOTAM,MEL,旅客记录,PNR,航线收益,客座率,飞行数据记录器,ACARS
- patterns-en: airworthiness,flight control parameter,FMS configuration,waypoint,NOTAM,MEL,passenger record,PNR,route yield,load factor,FDR,ACARS

### 20. 铁路/高铁 (railway)
- 入站: CTCS信号参数(block)、调度运行图(warn)、线路限速(warn)
- LLM-req: 查询信号配置(block)、修改线路参数(block)
- LLM-resp: 信号系统数据泄露(block)、运行图泄露(warn,regex: CTCS|列控|ATP|应答器|运行图)
- PathPolicy: tpl-railway → pp-002,pp-005,pp-007,pp-008
- patterns-cn: CTCS,列控系统,ATP参数,应答器,轨道电路,运行图,调度命令,线路限速,闭塞分区,联锁表,信号机
- patterns-en: CTCS,train control system,ATP parameter,balise,track circuit,timetable,dispatch command,speed restriction,block section,interlocking table,signal

### 21. 城市轨道/地铁 (metro)
- 入站: CBTC参数(block)、屏蔽门配置(warn)、客流数据(warn)
- LLM-req: 修改CBTC配置(block)、查询运营数据(warn)
- LLM-resp: CBTC数据泄露(block)、客流调度泄露(warn,regex: CBTC|屏蔽门|ATO|客流.*预测)
- PathPolicy: tpl-metro → pp-002,pp-005,pp-007,pp-008
- patterns-cn: CBTC,屏蔽门,ATO参数,站台门,客流预测,客流调度,列车自动运行,行车间隔,折返时间,应急疏散,正线运行
- patterns-en: CBTC,platform screen door,ATO parameter,passenger flow forecast,train automation,headway,turnaround time,emergency evacuation

### 22. 航运/港口 (maritime)
- 入站: AIS数据(warn)、海图数据(block)、港口调度(warn)
- LLM-req: 查询船舶位置(warn)、导出港口数据(warn)
- LLM-resp: 船舶数据泄露(warn)、港口调度泄露(warn,regex: AIS|MMSI|IMO.*number|泊位.*分配)
- PathPolicy: tpl-maritime → pp-004,pp-005,pp-011
- patterns-cn: AIS数据,船舶位置,MMSI,IMO编号,海图数据,港口调度,泊位分配,集装箱追踪,船期表,提单,报关单
- patterns-en: AIS data,vessel position,MMSI,IMO number,nautical chart,port schedule,berth allocation,container tracking,shipping schedule,bill of lading,customs declaration

## ====== 互联网细分 ======

### 23. 游戏 (gaming)
- 入站: 反外挂策略(block)、内购定价(warn)、虚拟资产(warn)
- LLM-req: 查询反外挂规则(block)、修改内购价格(block)
- LLM-resp: 反外挂策略泄露(block)、游戏经济数据泄露(warn,regex: 外挂.*检测|anti-cheat|充值.*比例|drop.*rate)
- PathPolicy: tpl-gaming → pp-004,pp-005,pp-011
- patterns-cn: 反外挂策略,外挂检测,游戏币,内购定价,充值比例,掉落概率,抽卡概率,未成年防沉迷,虚拟道具,服务器架构,游戏源码
- patterns-en: anti-cheat strategy,cheat detection,in-app purchase pricing,drop rate,gacha rate,minor protection,virtual item,server architecture,game source code

### 24. 广告/营销 (advertising)
- 入站: 用户标签(warn)、投放策略(warn)、竞品数据(warn)
- LLM-req: 导出用户画像(warn)、查询竞品投放(warn)
- LLM-resp: 投放策略泄露(warn)、DMP数据泄露(warn,regex: DMP|人群包|ROAS|CPA.*bid|conversion.*rate)
- PathPolicy: tpl-advertising → pp-003,pp-004,pp-011
- patterns-cn: 用户标签,人群包,DMP数据,投放策略,竞品监控,ROI数据,转化漏斗,出价策略,千次展示成本,广告素材库
- patterns-en: user tag,audience segment,DMP data,media plan,competitor monitoring,ROI data,conversion funnel,bidding strategy,CPM,creative library

### 25. 社交平台 (social-media)
- 入站: 用户关系链(block)、私信内容(block)、推荐算法(block)
- LLM-req: 导出用户关系(block)、查询推荐策略(block)
- LLM-resp: 关系链泄露(block)、推荐算法泄露(block,regex: 推荐.*算法|social.*graph|关系链|feed.*rank)
- PathPolicy: tpl-social-media → pp-004,pp-005,pp-009,pp-011,pp-012
- patterns-cn: 用户关系链,好友列表,私信内容,推荐算法,内容审核策略,举报数据,社交图谱,信息流排序,用户行为日志
- patterns-en: social graph,friend list,direct message,recommendation algorithm,content moderation policy,report data,feed ranking,user behavior log

### 26. 短视频/直播 (live-streaming)
- 入站: 主播收入(block)、流量分发(block)、MCN合约(warn)
- LLM-req: 查询主播收入(block)、导出分发规则(block)
- LLM-resp: 主播收入泄露(block)、流量规则泄露(warn,regex: 流量.*分发|主播.*分成|MCN.*合约|打赏.*比例)
- PathPolicy: tpl-live-streaming → pp-004,pp-005,pp-011,pp-012
- patterns-cn: 主播收入,打赏分成,流量分发规则,MCN合约,直播间权重,带货佣金,礼物分成比例,推流地址,直播源码,审核策略
- patterns-en: streamer revenue,gift sharing ratio,traffic distribution rule,MCN contract,live room weight,commission rate,streaming address,source code,moderation policy

### 27. SaaS/云服务 (saas-cloud)
- 入站: 客户数据(block)、多租户配置(warn)、API密钥(block)
- LLM-req: 查询客户数据(block)、导出租户配置(warn)
- LLM-resp: 客户数据泄露(block)、云配置泄露(warn,regex: tenant.*config|SLA.*breach|客户.*数据|access.*key)
- PathPolicy: tpl-saas-cloud → pp-004,pp-005,pp-009,pp-011,pp-012
- patterns-cn: 客户数据隔离,多租户配置,API密钥,服务可用性,SLA违约,客户续约率,ARR,MRR,客户流失率,部署架构
- patterns-en: customer data isolation,multi-tenant config,API key,service availability,SLA breach,customer retention,ARR,MRR,churn rate,deployment architecture

### 28. 搜索引擎 (search-engine)
- 入站: 搜索日志(warn)、排名算法(block)、广告竞价(warn)
- LLM-req: 查询排名规则(block)、导出搜索日志(warn)
- LLM-resp: 排名算法泄露(block)、搜索数据泄露(warn,regex: 排名.*算法|PageRank|索引.*策略|crawl.*policy)
- PathPolicy: tpl-search-engine → pp-004,pp-005,pp-011,pp-012
- patterns-cn: 搜索排名算法,排名因子,索引策略,爬虫策略,搜索日志,用户搜索词,广告竞价规则,质量得分,搜索意图
- patterns-en: ranking algorithm,ranking factor,indexing strategy,crawl policy,search log,search query,ad auction rule,quality score,search intent

### 29. 外卖/本地生活 (local-services)
- 入站: 骑手轨迹(warn)、商户评分算法(block)、用户地址(warn)
- LLM-req: 导出骑手数据(warn)、查询评分算法(block)
- LLM-resp: 商户数据泄露(warn)、配送数据泄露(warn,regex: 骑手.*轨迹|商户.*评分|配送.*路径|佣金.*比例)
- PathPolicy: tpl-local-services → pp-003,pp-004,pp-011
- patterns-cn: 骑手轨迹,配送路径,商户评分算法,佣金比例,抽成比例,商户流水,用户收货地址,运力调度,高峰定价,满减策略
- patterns-en: rider trajectory,delivery route,merchant scoring algorithm,commission rate,merchant revenue,delivery address,capacity scheduling,surge pricing,promotion strategy

### 30. 网络安全 (cybersecurity)
- 入站: 漏洞数据(block)、攻击payload(block)、0day信息(block)
- LLM-req: 查询未公开漏洞(block)、导出渗透报告(block)
- LLM-resp: 漏洞详情泄露(block)、客户资产泄露(block,regex: CVE-\d{4}-\d+|0day|exploit.*code|PoC|渗透.*报告)
- PathPolicy: tpl-cybersecurity → pp-004,pp-005,pp-009,pp-011,pp-012
- patterns-cn: 0day漏洞,未公开漏洞,攻击载荷,渗透报告,客户资产,漏洞利用代码,PoC代码,攻击工具链,安全审计报告,应急响应
- patterns-en: zero-day,undisclosed vulnerability,attack payload,penetration report,client asset,exploit code,PoC code,attack toolchain,security audit report,incident response

## ====== 其他行业 ======

### 31. 传媒/新闻 (media-news)
- 入站: 未发布稿件(block)、信息源(block)、独家线索(warn)
- LLM-req: 查询未发布内容(block)、暴露信息源(block)
- LLM-resp: 未发布内容泄露(block)、信息源泄露(block,regex: 信息源|anonymous.*source|独家|exclusive.*story)
- PathPolicy: tpl-media-news → pp-004,pp-005,pp-011,pp-012
- patterns-cn: 未发布稿件,信息源,匿名线人,独家新闻,采编策略,发稿计划,新闻素材,审稿流程,舆论引导,舆情监控
- patterns-en: unpublished article,anonymous source,confidential informant,exclusive story,editorial strategy,publication schedule,news material,editorial review,public opinion guidance

### 32. 出版/版权 (publishing)
- 入站: 未出版手稿(block)、版税数据(warn)、DRM配置(block)
- LLM-req: 导出手稿内容(block)、查询版税明细(warn)
- LLM-resp: 手稿内容泄露(block)、版税数据泄露(warn,regex: 版税|royalty.*rate|ISBN.*\d{13}|稿费)
- PathPolicy: tpl-publishing → pp-004,pp-005,pp-011
- patterns-cn: 未出版手稿,版税数据,稿费标准,DRM配置,数字版权,ISBN分配,印数,首印量,发行渠道,翻译合同
- patterns-en: unpublished manuscript,royalty data,author fee,DRM configuration,digital rights,ISBN assignment,print run,distribution channel,translation contract

### 33. 电信/运营商 (telecom)
- 入站: 通话记录CDR(block)、基站数据(block)、用户号码(warn)
- LLM-req: 查询通话记录(block)、导出基站数据(block)
- LLM-resp: CDR数据泄露(block)、网络拓扑泄露(warn,regex: CDR|基站.*位置|IMSI|IMEI|核心网)
- PathPolicy: tpl-telecom → pp-004,pp-005,pp-009,pp-011,pp-012
- patterns-cn: 通话记录,CDR数据,基站位置,信令数据,IMSI,IMEI,核心网配置,用户套餐,号码归属,监听接口,DPI数据
- patterns-en: call detail record,CDR data,base station location,signaling data,IMSI,IMEI,core network config,subscriber plan,number ownership,lawful interception,DPI data

### 34. 物流/供应链 (logistics)
- 入站: 客户地址(warn)、仓储布局(warn)、供应商报价(block)
- LLM-req: 导出客户地址(warn)、查询供应商报价(block)
- LLM-resp: 物流路由泄露(warn)、供应链数据泄露(warn,regex: 仓储.*布局|供应商.*报价|物流.*路由|库存.*数据)
- PathPolicy: tpl-logistics → pp-003,pp-004,pp-011
- patterns-cn: 客户收货地址,仓储布局,库位规划,供应商报价,物流路由,运费协议,库存数据,入库单,出库单,供应链金融
- patterns-en: customer shipping address,warehouse layout,storage planning,supplier quotation,logistics route,freight agreement,inventory data,supply chain finance

### 35. 房地产/物业 (real-estate)
- 入站: 业主信息(warn)、房价数据(warn)、户型图纸(warn)
- LLM-req: 导出业主数据(warn)、查询成交底价(block)
- LLM-resp: 业主信息泄露(warn)、交易数据泄露(warn,regex: 业主.*信息|成交.*底价|楼盘.*均价|物业.*费)
- PathPolicy: tpl-real-estate → pp-004,pp-011
- patterns-cn: 业主信息,业主名单,房价数据,成交底价,户型图纸,楼盘均价,物业费,公摊面积,购房合同,按揭数据
- patterns-en: owner information,owner list,property price,transaction floor price,floor plan,average price,property fee,mortgage data,purchase contract

### 36. 农业/食品 (agriculture)
- 入站: 种子专利(block)、农药配方(block)、溯源数据(warn)
- LLM-req: 查询种子配方(block)、导出溯源数据(warn)
- LLM-resp: 种子专利泄露(block)、食品检测数据泄露(warn,regex: 种子.*专利|转基因|农药.*配方|食品.*检测)
- PathPolicy: tpl-agriculture → pp-004,pp-005,pp-011
- patterns-cn: 种子专利,转基因数据,育种记录,农药配方,农药残留,食品安全检测,溯源数据,产地证明,有机认证,饲料配方
- patterns-en: seed patent,GMO data,breeding record,pesticide formula,pesticide residue,food safety test,traceability data,origin certificate,organic certification,feed formula

### 37. 航空航天 (aerospace)
- 入站: ITAR管制(block)、卫星参数(block)、飞控代码(block)
- LLM-req: 查询卫星轨道(warn)、导出飞控代码(block)
- LLM-resp: 卫星参数泄露(block)、航天数据泄露(block,regex: ITAR|TLE|轨道.*参数|遥测|星载)
- PathPolicy: tpl-aerospace → pp-002,pp-005,pp-007,pp-008,pp-011,pp-012
- patterns-cn: ITAR管制,卫星参数,轨道数据,TLE,遥测数据,遥控指令,星载软件,火箭参数,发射窗口,测控频段,载荷参数
- patterns-en: ITAR controlled,satellite parameter,orbital data,TLE,telemetry data,telecommand,onboard software,rocket parameter,launch window,tracking frequency,payload parameter

### 38. 矿业/资源 (mining)
- 入站: 勘探数据(block)、矿藏储量(block)、环评数据(warn)
- LLM-req: 查询矿藏数据(block)、导出勘探报告(block)
- LLM-resp: 勘探数据泄露(block)、储量数据泄露(block,regex: 勘探.*数据|矿藏.*储量|品位|exploration.*data)
- PathPolicy: tpl-mining → pp-004,pp-005,pp-011,pp-012
- patterns-cn: 勘探数据,矿藏储量,品位数据,采矿权,探矿权,环评报告,尾矿处理,选矿参数,矿石成分,地质报告
- patterns-en: exploration data,mineral reserves,ore grade,mining rights,prospecting rights,environmental assessment,tailings,beneficiation parameter,ore composition,geological report

### 39. 建筑/工程 (construction)
- 入站: 设计图纸(warn)、结构计算(warn)、招标底价(block)
- LLM-req: 查询招标底价(block)、导出设计图纸(warn)
- LLM-resp: 设计数据泄露(warn)、招标数据泄露(block,regex: 招标.*底价|工程.*造价|投标.*报价|BIM.*模型)
- PathPolicy: tpl-construction → pp-004,pp-005,pp-011
- patterns-cn: 设计图纸,结构计算书,施工方案,招标底价,投标报价,工程造价,BIM模型,地勘报告,变更单,签证单,结算审计
- patterns-en: design drawing,structural calculation,construction plan,bid floor price,tender price,project cost,BIM model,geotechnical report,change order,site instruction,final account

### 40 (bonus). 酒店/旅游 (hospitality)
- 入站: 旅客信息PNR(warn)、VIP客户(warn)、定价策略(block)
- LLM-req: 导出旅客数据(warn)、查询收益管理策略(block)
- LLM-resp: 客户数据泄露(warn)、定价策略泄露(warn,regex: RevPAR|ADR|OCC.*rate|收益.*管理|房价.*策略)
- PathPolicy: tpl-hospitality → pp-004,pp-011
- patterns-cn: 旅客信息,PNR记录,VIP客户,定价策略,收益管理,房价策略,入住率,RevPAR,客户偏好,会员数据
- patterns-en: guest information,PNR record,VIP customer,pricing strategy,revenue management,rate strategy,occupancy rate,RevPAR,customer preference,loyalty data
