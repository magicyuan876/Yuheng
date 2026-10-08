import { readFileSync, readdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { defineConfig, type DefaultTheme } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'
import './theme-config.d.ts'
import { repoVersionLabel } from './version'

const root = resolve(import.meta.dirname, '..')

const sections: { dir: string; label: string }[] = [
  { dir: '01-getting-started', label: '快速开始' },
  { dir: '02-architecture', label: '架构' },
  { dir: '03-features', label: '功能模块' },
  { dir: '04-api', label: 'API 参考' },
  { dir: '05-clients', label: '客户端' },
  { dir: '06-development', label: '开发指南' },
]

/** 侧边栏条目文字：取正文一级标题，去掉冗余前后缀 */
function itemText(dir: string, file: string): string {
  const raw = readFileSync(resolve(root, dir, file), 'utf-8')
  const heading = raw.match(/^#\s+(.+)$/m)?.[1] ?? file.replace(/\.md$/, '')
  return heading
    .replace(/^API 参考[:：]\s*/, '')
    .replace(/\s*[（(][^（()）]*[)）]\s*/g, ' ')
    .replace(/\s{2,}/g, ' ')
    .trim()
}

function itemsOf(dir: string): DefaultTheme.SidebarItem[] {
  return readdirSync(resolve(root, dir))
    .filter((f) => f.endsWith('.md'))
    .sort()
    .map((f) => ({
      text: itemText(dir, f),
      link: `/${dir}/${f.replace(/\.md$/, '')}`,
    }))
}

const sidebar: DefaultTheme.SidebarItem[] = sections.map((s) => ({
  text: s.label,
  collapsed: false,
  items: itemsOf(s.dir),
}))

/** 本地搜索默认按空白分词，中文整段会被当作一个词，这里退化为字粒度切分 */
function tokenize(text: string): string[] {
  const tokens: string[] = []
  for (const part of text.split(/[\s\n\r#%*,=/:;?[\]{}()&+\-!'"$·、，。：；？！（）【】《》…—]+/)) {
    if (!part) continue
    if (/[\u4e00-\u9fa5]/.test(part)) {
      tokens.push(part, ...part.split(''))
    } else {
      tokens.push(part)
    }
  }
  return tokens
}

const repo = 'https://github.com/magicyuan876/Yuheng'
const site = 'https://magicyuan876.github.io/Yuheng'

// The site is published in two places under different prefixes: GitHub Pages
// serves it under the repository name (/Yuheng/, set by the docs workflow), and
// the Docker image serves it under /docs/ behind a gateway (the default here,
// which website-docs/nginx.conf expects).
const base = (process.env.DOCS_BASE ?? '/docs/') as `/${string}/`

export default withMermaid(
  defineConfig({
    title: 'Yuheng',
    titleTemplate: ':title · Yuheng 文档',
    description: 'Yuheng（玉衡），AI 智能体时代的知识平台：部署、配置、功能说明、API 参考与二次开发',
    lang: 'zh-CN',

    // Chinese is the primary language and lives at the site root. English exists
    // only for the installation and quick start pages, under /en/.
    locales: {
      root: { label: '简体中文', lang: 'zh-CN' },
      en: {
        label: 'English',
        lang: 'en-US',
        link: '/en/',
        titleTemplate: ':title · Yuheng docs',
        description: 'Yuheng documentation in English: installation and quick start. The rest of the documentation is in Chinese.',
        themeConfig: {
          nav: [
            { text: 'Installation', link: '/en/01-getting-started/02-installation', activeMatch: '/en/01-getting-started/02' },
            { text: 'Quick start', link: '/en/01-getting-started/03-quickstart', activeMatch: '/en/01-getting-started/03' },
            { text: '中文文档', link: '/01-getting-started/01-introduction' },
          ],
          sidebar: [
            {
              text: 'Getting started',
              items: [
                { text: 'About this translation', link: '/en/' },
                { text: 'Installation', link: '/en/01-getting-started/02-installation' },
                { text: 'Quick start', link: '/en/01-getting-started/03-quickstart' },
              ],
            },
          ],
          outline: { level: [2, 3], label: 'On this page' },
          docFooter: { prev: 'Previous', next: 'Next' },
          returnToTopLabel: 'Back to top',
          sidebarMenuLabel: 'Menu',
          darkModeSwitchLabel: 'Appearance',
          lightModeSwitchTitle: 'Switch to light theme',
          darkModeSwitchTitle: 'Switch to dark theme',
          langMenuLabel: 'Languages',
          lastUpdated: { text: 'Last updated', formatOptions: { dateStyle: 'medium', timeStyle: undefined } },
          editLink: { pattern: `${repo}/edit/main/website-docs/:path`, text: 'Edit this page on GitHub' },
        },
      },
    },
    base,
    cleanUrls: true,
    lastUpdated: true,
    srcExclude: ['README.md'],
    metaChunk: true,

    head: [
      ['link', { rel: 'icon', href: `${base}favicon.svg`, type: 'image/svg+xml' }],
      ['meta', { name: 'theme-color', content: '#101f38' }],
      ['meta', { property: 'og:image', content: `${site}/brand/yuheng-banner.png` }],
      ['meta', { property: 'og:type', content: 'website' }],
      ['meta', { property: 'og:title', content: 'Yuheng 文档' }],
      [
        'meta',
        {
          property: 'og:description',
          content: 'AI 智能体时代的知识平台：接入资料与在线文档，带出处的问答，知识健康持续维护，REST 与 MCP 开放给智能体',
        },
      ],
    ],

    markdown: {
      theme: { light: 'github-light', dark: 'github-dark' },
      lineNumbers: false,
      toc: { level: [2, 3] },
      config(md) {
        // 表格外面包一层滚动容器。默认主题把 <table> 本身设成 display:block 来做
        // 横向滚动，副作用是表格按内容收缩——列少的表比正文列窄一截，页面里宽窄不一。
        // 把滚动交给包裹层后，表格可以恢复 display:table + width:100%，统一撑满正文列。
        md.renderer.rules.table_open = () => '<div class="wk-table">\n<table>\n'
        md.renderer.rules.table_close = () => '</table>\n</div>\n'
      },
    },

    themeConfig: {
      logo: { light: '/logo-mark.svg', dark: '/logo-mark-dark.svg', alt: 'Yuheng' },
      siteTitle: 'Yuheng',

      nav: [
        { text: '快速开始', link: '/01-getting-started/01-introduction', activeMatch: '/01-getting-started/' },
        { text: '架构', link: '/02-architecture/01-overview', activeMatch: '/02-architecture/' },
        { text: '功能', link: '/03-features/01-tenant-auth', activeMatch: '/03-features/' },
        { text: 'API', link: '/04-api/01-api-overview', activeMatch: '/04-api/' },
        { text: '客户端', link: '/05-clients/01-frontend', activeMatch: '/05-clients/' },
        { text: '开发', link: '/06-development/01-dev-guide', activeMatch: '/06-development/' },
      ],

      yuhengVersion: repoVersionLabel,

      sidebar,

      socialLinks: [{ icon: 'github', link: repo }],

      outline: { level: [2, 3], label: '本页目录' },

      docFooter: { prev: '上一篇', next: '下一篇' },
      returnToTopLabel: '回到顶部',
      sidebarMenuLabel: '目录',
      darkModeSwitchLabel: '外观',
      lightModeSwitchTitle: '切换到浅色',
      darkModeSwitchTitle: '切换到深色',

      lastUpdated: {
        text: '最后更新',
        formatOptions: { dateStyle: 'medium', timeStyle: undefined },
      },

      editLink: {
        pattern: `${repo}/edit/main/website-docs/:path`,
        text: '在 GitHub 上编辑此页',
      },

      search: {
        provider: 'local',
        options: {
          locales: {
            en: {
              translations: {
                button: { buttonText: 'Search', buttonAriaLabel: 'Search' },
                modal: {
                  displayDetails: 'Display detailed list',
                  resetButtonTitle: 'Reset',
                  backButtonTitle: 'Close',
                  noResultsText: 'No results for',
                  footer: { selectText: 'to select', navigateText: 'to navigate', closeText: 'to close' },
                },
              },
            },
          },
          translations: {
            button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
            modal: {
              displayDetails: '展开详情',
              resetButtonTitle: '清除',
              backButtonTitle: '返回',
              noResultsText: '没有找到结果',
              footer: {
                selectText: '选择',
                navigateText: '切换',
                closeText: '关闭',
              },
            },
          },
          miniSearch: {
            options: { tokenize },
            searchOptions: {
              fuzzy: 0.2,
              prefix: true,
              boost: { title: 4, text: 2, titles: 1 },
            },
          },
        },
      },

      footer: {
        message: `基于 Yuheng ${repoVersionLabel} 源码整理 · MIT License`,
        copyright: 'Portions © 2025 Tencent · Modifications © 2026 magicyuan876 · MIT',
      },
    },

    mermaid: {
      theme: 'base',
      fontFamily:
        '"PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", ui-sans-serif, sans-serif',
      themeVariables: {
        primaryColor: '#eef1f6',
        primaryTextColor: '#101f38',
        primaryBorderColor: '#9aa8bd',
        lineColor: '#7d8ba1',
        secondaryColor: '#faf7ef',
        tertiaryColor: '#f6f7f9',
        fontSize: '14px',
      },
    },
  }),
)
