export const STAGES = [
  ['INIT', '立项评估'],
  ['VOTING', '楼栋表决'],
  ['SUBSCRIBED', '认购缴费'],
  ['SCHEDULED', '施工排期'],
  ['GRID_CONNECTED', '并网验收'],
  ['OPERATING', '发电运营'],
]

export const SUBSCRIPTION_STATUS = { PENDING: '待缴费', PAID: '已缴费', REFUNDED: '已退款' }
export const SUB_STATUS_TONE = { PENDING: 'amber', PAID: 'green', REFUNDED: 'gray' }

export const DEVICE_STATUS = { NORMAL: '正常', FAULT: '故障', REPAIRED: '已修复' }
export const DEVICE_STATUS_TONE = { NORMAL: 'green', FAULT: 'red', REPAIRED: 'blue' }

export const REPAIR_CATEGORY = { INVERTER_FAULT: '逆变器故障', LEAK: '屋顶漏水', OTHER: '其他' }
export const REPAIR_STATUS = { OPEN: '处理中', RESOLVED: '已解决', CLOSED: '已关闭' }
export const REPAIR_STATUS_TONE = { OPEN: 'red', RESOLVED: 'green', CLOSED: 'gray' }
export const RESPONSIBILITY = {
  PROPERTY: '物业', SUPPLIER: '设备供应商', INSTALLER: '施工方', RESIDENT: '住户', PUBLIC: '公共(全体业主)',
}

export const SETTLE_STATUS = { DRAFT: '草稿', CONFIRMED: '已确认入账', ADJUSTED: '已调整' }
export const SETTLE_TONE = { DRAFT: 'amber', CONFIRMED: 'green', ADJUSTED: 'blue' }

export const SCHEDULE_KIND = {
  PLAN: '计划', DELAY: '供应延迟', GRID_CHANGE: '并网变更', MILESTONE: '里程碑',
}
export const SCHEDULE_KIND_TONE = {
  PLAN: 'blue', DELAY: 'red', GRID_CHANGE: 'amber', MILESTONE: 'green',
}

export const TX_TYPE = {
  INIT: '账户开立', SUBSCRIPTION: '认购本金归集', SUBSIDY: '补贴', SETTLEMENT: '发电结算', REPAIR: '维修支出',
  MAINTENANCE: '运维费用', TAX: '税费', REFUND: '退款', COMPENSATION: '责任赔偿',
  BENEFIT: '收益发放', TRANSFER: '转让',
}

export const VOTE_LABEL = { APPROVE: '赞成', REJECT: '反对', ABSTAIN: '弃权' }
export const VOTE_TONE = { APPROVE: 'green', REJECT: 'red', ABSTAIN: 'gray' }

export const OBJECTION_STATUS = {
  OPEN: '受理中', VOTING: '投票中', APPROVED: '异议成立', REJECTED: '异议驳回', CLOSED: '已关闭',
}
export const OBJECTION_TONE = {
  OPEN: 'amber', VOTING: 'blue', APPROVED: 'red', REJECTED: 'gray', CLOSED: 'gray',
}

export const TRANSFER_STATUS = { PENDING: '待处理', TRANSFERRED: '已完成', SETTLED: '已结清', REFUNDED: '已退款' }
export const TRANSFER_TONE = { PENDING: 'amber', TRANSFERRED: 'green', SETTLED: 'blue', REFUNDED: 'gray' }
