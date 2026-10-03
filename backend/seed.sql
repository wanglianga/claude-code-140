-- 种子数据(幂等)
INSERT INTO admins (username,password,name) VALUES ('admin','admin123','平台管理员')
ON CONFLICT (username) DO NOTHING;

INSERT INTO buildings
(code,name,address,rooftop_area,orientation,load_assessment,shading,property_rights,
public_benefit_rule,grid_condition,construction_impact,total_capacity_kw,component_count,
component_unit_price,public_share_pct,vote_threshold_pct,electricity_price,subsidy_rate,
maintenance_fee_pct,tax_pct,repair_fund_pct,stage)
VALUES
('B-001','阳光里3栋','阳光里小区3栋',480.00,'正南偏东8°','原设计活荷载2.0kN/m²,经检测满足光伏附加荷载0.35kN/m²',
 '上午9点前东侧女儿墙局部遮挡,约影响3%组件',
 '屋面为全体业主共有;构筑物及管线归属物业,加装部分按份共有',
 '公共部分收益的20%计入楼栋公共账户,优先用于屋面维修与公共电费,年底公示余额',
 '小区配电室剩余容量120kW,拟100kW全额上网,并网点在低压计量柜,电网已出具初步接入意见',
 '施工期3周,需保留消防通道与检修通道,材料夜间不堆放屋面,顶楼住户阳台侧搭设防护',
 100.00,200,1200.00,20.0,67.0,0.3900,0.0500,5.0,3.0,3.0,'INIT')
ON CONFLICT (code) DO NOTHING;

SELECT setval('buildings_id_seq', (SELECT MAX(id) FROM buildings));

-- 公共账户开立
INSERT INTO public_transactions (building_id,type,direction,amount,balance_after,remark)
SELECT b.id,'INIT','IN',0,0,'公共账户开立' FROM buildings b
WHERE b.code='B-001'
  AND NOT EXISTS (SELECT 1 FROM public_transactions pt WHERE pt.building_id=b.id);

-- 住户(密码统一 123456, 物业账号 role=PROPERTY)
INSERT INTO residents (building_id,unit_no,owner_name,phone,role,area_owned,is_top_floor,password)
SELECT b.id, v.unit_no, v.name, v.phone, v.role, v.area, v.top, '123456'
FROM (VALUES
  ('101','王物业','13800000001','PROPERTY',88.50,FALSE),
  ('102','李建国','13800000002','RESIDENT',88.50,FALSE),
  ('201','张敏','13800000003','RESIDENT',92.00,FALSE),
  ('202','陈晓东','13800000004','RESIDENT',92.00,FALSE),
  ('301','刘洋','13800000005','RESIDENT',90.20,FALSE),
  ('302','赵芳','13800000006','RESIDENT',90.20,FALSE),
  ('401','孙志强','13800000007','RESIDENT',89.80,FALSE),
  ('402','周丽华','13800000008','RESIDENT',89.80,FALSE),
  ('501','吴海涛','13800000009','RESIDENT',91.00,FALSE),
  ('502','郑海燕','13800000010','RESIDENT',91.00,FALSE),
  ('601','钱进','13800000011','RESIDENT',92.00,TRUE),
  ('602','冯雪','13800000012','RESIDENT',92.00,TRUE)
) AS v(unit_no,name,phone,role,area,top)
CROSS JOIN buildings b
WHERE b.code='B-001'
ON CONFLICT (building_id,unit_no) DO NOTHING;

-- 初始公告
INSERT INTO announcements (building_id,title,content,created_by)
SELECT b.id,'屋顶光伏项目意向征集公告',
'各位业主:拟在本栋屋面建设100kW分布式光伏,组件200块。请关注承重评估、遮挡与公共收益规则,并在表决阶段投票。顶楼住户如担心漏水可在异议区提出。',
(SELECT id FROM residents r WHERE r.building_id=b.id AND r.unit_no='101')
FROM buildings b WHERE b.code='B-001'
  AND NOT EXISTS (SELECT 1 FROM announcements a WHERE a.building_id=b.id);
