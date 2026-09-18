// The emoji picker behind the ":" trigger.
//
// A curated list rather than a dependency. The full Unicode set is several
// thousand entries and a megabyte of names, none of which a documentation
// tool needs: what people actually reach for in a design note or a status
// update is a small, stable set, and shipping it as data costs nothing and
// loads instantly. Extend the list as real pages want more.
//
// Each entry carries both English and Chinese keywords, so the picker answers
// to whichever language somebody is writing in rather than to whichever the
// interface happens to be set to.

/** One entry in the picker. */
export interface Emoji {
  /** The character itself, which is what gets inserted. */
  char: string
  /** A short name, matched first and shown beside the character. */
  name: string
  /** Other words that should find it, in both languages. */
  keywords: string[]
}

/** How many entries a menu shows at once. */
export const MAX_EMOJI_RESULTS = 24

/** The shortest query worth searching, so ":" alone does not open a wall. */
export const MIN_EMOJI_QUERY = 1

const e = (char: string, name: string, ...keywords: string[]): Emoji => ({ char, name, keywords })

export const EMOJI: readonly Emoji[] = [
  // Reactions and faces
  e('👍', 'thumbsup', 'yes', 'ok', 'approve', 'like', '赞', '好', '同意'),
  e('👎', 'thumbsdown', 'no', 'reject', '踩', '反对'),
  e('👏', 'clap', 'applause', 'well done', '鼓掌', '厉害'),
  e('🙏', 'pray', 'thanks', 'please', '感谢', '拜托'),
  e('🙌', 'raised hands', 'celebrate', '欢呼'),
  e('🤝', 'handshake', 'agree', 'deal', '握手', '合作'),
  e('💪', 'muscle', 'strong', '加油', '努力'),
  e('👀', 'eyes', 'looking', 'review', '看', '关注'),
  e('🧠', 'brain', 'think', 'idea', '大脑', '思考'),
  e('😀', 'grinning', 'happy', 'smile', '笑', '开心'),
  e('😄', 'smile', 'happy', '微笑'),
  e('😅', 'sweat smile', 'awkward', '尴尬', '苦笑'),
  e('😂', 'joy', 'laugh', 'lol', '大笑'),
  e('🙂', 'slight smile', '微笑'),
  e('😉', 'wink', '眨眼'),
  e('😍', 'heart eyes', 'love', '喜欢'),
  e('🤔', 'thinking', 'hmm', 'question', '思考', '疑问'),
  e('😐', 'neutral', 'meh', '面无表情'),
  e('😴', 'sleeping', 'tired', '睡觉', '困'),
  e('😭', 'sob', 'cry', '哭'),
  e('😱', 'scream', 'shock', '震惊'),
  e('😎', 'sunglasses', 'cool', '酷'),
  e('🤯', 'mind blown', 'wow', '震撼'),
  e('🥳', 'partying', 'celebrate', '庆祝'),
  e('😇', 'innocent', '天使'),
  e('🤖', 'robot', 'bot', 'ai', '机器人'),
  e('👋', 'wave', 'hello', 'bye', '你好', '再见'),
  e('🫡', 'salute', 'yes sir', '敬礼', '收到'),

  // Status and process
  e('✅', 'check', 'done', 'complete', 'yes', '完成', '通过'),
  e('❌', 'cross', 'no', 'fail', 'wrong', '失败', '错误'),
  e('⚠️', 'warning', 'caution', '警告', '注意'),
  e('🚧', 'construction', 'wip', 'in progress', '施工', '进行中'),
  e('🔥', 'fire', 'hot', 'urgent', '火', '紧急', '热门'),
  e('⭐', 'star', 'favourite', '星', '收藏'),
  e('🎯', 'target', 'goal', 'aim', '目标'),
  e('🚀', 'rocket', 'launch', 'ship', 'release', '发布', '上线'),
  e('🐛', 'bug', 'defect', 'issue', '缺陷', '问题'),
  e('🔧', 'wrench', 'fix', 'repair', '修复', '工具'),
  e('🛠️', 'tools', 'build', '构建', '工具'),
  e('⚙️', 'gear', 'settings', 'config', '设置', '配置'),
  e('🔒', 'lock', 'secure', 'private', '锁', '安全'),
  e('🔑', 'key', 'access', 'secret', '密钥', '权限'),
  e('🔍', 'search', 'find', 'magnifier', '搜索', '查找'),
  e('📌', 'pin', 'important', '置顶', '重要'),
  e('📍', 'location', 'place', '位置'),
  e('🏷️', 'label', 'tag', '标签'),
  e('🔗', 'link', 'url', '链接'),
  e('⏰', 'alarm', 'deadline', 'time', '闹钟', '截止'),
  e('⏳', 'hourglass', 'waiting', 'pending', '等待', '进行中'),
  e('📅', 'calendar', 'date', 'schedule', '日历', '日程'),
  e('🔔', 'bell', 'notify', 'reminder', '通知', '提醒'),
  e('🔕', 'mute', 'silence', '静音'),
  e('♻️', 'recycle', 'refactor', 'reuse', '重构', '回收'),
  e('🧹', 'broom', 'cleanup', 'chore', '清理'),
  e('🩹', 'bandage', 'patch', 'hotfix', '补丁'),
  e('⚡', 'zap', 'fast', 'performance', '性能', '快'),
  e('🐢', 'turtle', 'slow', '慢'),
  e('🧪', 'test tube', 'test', 'experiment', '测试', '实验'),
  e('🧭', 'compass', 'direction', 'guide', '方向', '指南'),

  // Documents and data
  e('📄', 'page', 'document', 'file', '文档', '页面'),
  e('📝', 'memo', 'note', 'write', 'draft', '笔记', '草稿'),
  e('📋', 'clipboard', 'copy', 'list', '剪贴板', '清单'),
  e('📁', 'folder', 'directory', '文件夹', '目录'),
  e('📦', 'package', 'release', 'box', '包', '发布'),
  e('📊', 'bar chart', 'stats', 'data', '图表', '数据'),
  e('📈', 'chart up', 'growth', 'increase', '增长', '上升'),
  e('📉', 'chart down', 'decline', 'decrease', '下降'),
  e('🗂️', 'dividers', 'organise', '归档', '分类'),
  e('🗃️', 'file box', 'archive', '存档'),
  e('🗑️', 'wastebasket', 'delete', 'remove', '删除', '垃圾桶'),
  e('📚', 'books', 'docs', 'reference', '资料', '书'),
  e('📖', 'book', 'read', 'guide', '阅读', '手册'),
  e('✏️', 'pencil', 'edit', 'write', '编辑', '修改'),
  e('🖊️', 'pen', 'sign', '签名'),
  e('🖼️', 'picture', 'image', '图片'),
  e('🎬', 'clapper', 'video', 'film', '视频'),
  e('🎤', 'microphone', 'audio', 'record', '音频', '录音'),
  e('📎', 'paperclip', 'attachment', '附件'),
  e('✂️', 'scissors', 'cut', 'trim', '剪切'),

  // Technical
  e('💻', 'laptop', 'code', 'computer', '电脑', '代码'),
  e('🖥️', 'desktop', 'monitor', 'server', '显示器'),
  e('🗄️', 'cabinet', 'database', 'storage', '数据库', '存储'),
  e('☁️', 'cloud', 'saas', '云'),
  e('🌐', 'globe', 'web', 'internet', 'network', '网络', '国际化'),
  e('📡', 'satellite', 'signal', 'api', '信号'),
  e('🔌', 'plug', 'plugin', 'integration', '插件', '集成'),
  e('🧩', 'puzzle', 'module', 'component', '模块', '组件'),
  e('🏗️', 'crane', 'architecture', 'build', '架构', '建设'),
  e('🪝', 'hook', 'webhook', '钩子'),
  e('📥', 'inbox', 'import', 'receive', '导入', '收件'),
  e('📤', 'outbox', 'export', 'send', '导出', '发送'),
  e('🔄', 'refresh', 'sync', 'retry', '同步', '刷新'),
  e('↩️', 'undo', 'revert', 'back', '撤销', '回退'),
  e('🚫', 'prohibited', 'blocked', 'denied', '禁止', '阻断'),
  e('💥', 'boom', 'crash', 'breaking', '崩溃', '破坏性'),
  e('🩺', 'stethoscope', 'health', 'diagnose', '健康', '诊断'),
  e('📐', 'triangle ruler', 'design', 'spec', '设计', '规范'),

  // People and communication
  e('💬', 'speech', 'comment', 'chat', '评论', '聊天'),
  e('💡', 'bulb', 'idea', 'suggestion', '想法', '建议'),
  e('❓', 'question', 'ask', '问题', '疑问'),
  e('❗', 'exclamation', 'important', '重要', '注意'),
  e('📣', 'megaphone', 'announce', 'notice', '公告', '宣布'),
  e('🗳️', 'ballot', 'vote', 'decide', '投票', '决策'),
  e('👥', 'people', 'team', 'users', '团队', '用户'),
  e('🧑‍💻', 'developer', 'engineer', '开发', '工程师'),
  e('🎉', 'tada', 'celebrate', 'launch', '庆祝', '完成'),
  e('🏆', 'trophy', 'win', 'achievement', '奖杯', '成就'),
  e('❤️', 'heart', 'love', '爱心'),
  e('☕', 'coffee', 'break', '咖啡', '休息'),
]

/**
 * Filters the list against what has been typed after the colon.
 *
 * Ranked the same way the slash menu is — a match at the start of the name
 * before one at the start of a keyword, before one anywhere — for the same
 * reason: typing ":ch" should offer "check" before "chart down", and the
 * entry whose name begins that way is nearly always the one meant.
 *
 * A query shorter than MIN_EMOJI_QUERY returns nothing at all, so a colon
 * typed in the middle of a sentence does not open a menu over the text.
 */
export function matchEmoji(
  query: string,
  list: readonly Emoji[] = EMOJI,
  limit = MAX_EMOJI_RESULTS,
): Emoji[] {
  const needle = query.trim().toLowerCase()
  if (needle.length < MIN_EMOJI_QUERY) return []

  const scored: { emoji: Emoji; rank: number; at: number }[] = []
  list.forEach((emoji, at) => {
    const rank = rankOf(emoji, needle)
    if (rank >= 0) scored.push({ emoji, rank, at })
  })
  scored.sort((a, b) => (a.rank - b.rank) || (a.at - b.at))
  return scored.slice(0, limit).map((s) => s.emoji)
}

function rankOf(emoji: Emoji, needle: string): number {
  const name = emoji.name.toLowerCase()
  if (name === needle) return 0
  if (name.startsWith(needle)) return 1
  if (emoji.keywords.some((k) => k.toLowerCase().startsWith(needle))) return 2
  if (name.includes(needle)) return 3
  if (emoji.keywords.some((k) => k.toLowerCase().includes(needle))) return 4
  return -1
}
