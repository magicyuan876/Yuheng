<template>
  <div class="mx-auto max-w-[860px] px-6 py-8 [font-family:var(--app-font-family)]">
    <h1 class="mb-1 text-[24px] font-bold">Markdown Rendering Test</h1>
    <p class="text-muted-foreground mb-8 text-[14px]">
      Dev-only page for visual regression testing of chat answer markdown (same typography as botmsg). Add new test
      cases or paste arbitrary markdown in the editor below.
    </p>

    <!-- Basic Text Styles (GPT markdown test doc alignment) -->
    <section class="border-border mb-9 border-b border-solid pb-6">
      <h2 class="mb-3 text-[18px] font-semibold">Basic Text Styles</h2>
      <div class="my-3">
        <div
          class="markdown-content chat-markdown-typography chat-citation-pills border-border bg-card rounded-md border border-solid px-3 py-2"
          v-html="basicTextHtml"
        ></div>
      </div>
    </section>

    <!-- LaTeX Formulas -->
    <section class="border-border mb-9 border-b border-solid pb-6">
      <h2 class="mb-3 text-[18px] font-semibold">LaTeX Formulas</h2>
      <div v-for="(tc, i) in latexCases" :key="'latex-' + i" class="my-3">
        <div class="bg-muted mb-1.5 overflow-x-auto rounded px-2.5 py-1.5 text-[13px]">
          <code class="break-all whitespace-pre-wrap">{{ tc.raw }}</code>
        </div>
        <div
          class="markdown-content chat-markdown-typography chat-citation-pills border-border bg-card rounded-md border border-solid px-3 py-2"
          v-html="tc.html"
        ></div>
      </div>
    </section>

    <!-- Code Blocks -->
    <section class="border-border mb-9 border-b border-solid pb-6">
      <h2 class="mb-3 text-[18px] font-semibold">Code Blocks</h2>
      <div class="my-3">
        <div
          class="markdown-content chat-markdown-typography chat-citation-pills border-border bg-card rounded-md border border-solid px-3 py-2"
          v-html="codeBlockHtml"
        ></div>
      </div>
    </section>

    <!-- Tables -->
    <section class="border-border mb-9 border-b border-solid pb-6">
      <h2 class="mb-3 text-[18px] font-semibold">Tables</h2>
      <div class="my-3">
        <div
          class="markdown-content chat-markdown-typography chat-citation-pills border-border bg-card rounded-md border border-solid px-3 py-2"
          v-html="tableHtml"
        ></div>
      </div>
    </section>

    <!-- Lists & Blockquotes -->
    <section class="border-border mb-9 border-b border-solid pb-6">
      <h2 class="mb-3 text-[18px] font-semibold">Lists &amp; Blockquotes</h2>
      <div class="my-3">
        <div
          class="markdown-content chat-markdown-typography chat-citation-pills border-border bg-card rounded-md border border-solid px-3 py-2"
          v-html="listsHtml"
        ></div>
      </div>
    </section>

    <!-- Mixed Content (LaTeX + code + text) -->
    <section class="border-border mb-9 border-b border-solid pb-6">
      <h2 class="mb-3 text-[18px] font-semibold">Mixed Content</h2>
      <div class="my-3">
        <div
          class="markdown-content chat-markdown-typography chat-citation-pills border-border bg-card rounded-md border border-solid px-3 py-2"
          v-html="mixedHtml"
        ></div>
      </div>
    </section>

    <!-- Mermaid -->
    <section class="border-border mb-9 border-b border-solid pb-6">
      <h2 class="mb-3 text-[18px] font-semibold">Mermaid Diagram</h2>
      <div class="my-3">
        <div
          ref="mermaidContainer"
          class="markdown-content chat-markdown-typography chat-citation-pills border-border bg-card rounded-md border border-solid px-3 py-2"
          v-html="mermaidHtml"
        ></div>
      </div>
    </section>

    <!-- Streaming Simulation -->
    <section class="border-border mb-9 border-b border-solid pb-6">
      <h2 class="mb-3 text-[18px] font-semibold">Streaming Simulation</h2>
      <p class="text-muted-foreground mb-2 text-[13px]">
        Simulates character-by-character streaming, like during a chat response.
      </p>
      <div class="mb-3 flex items-center gap-3">
        <button
          @click="startStream"
          :disabled="isStreaming"
          data-slot="dev-button"
          class="border-border bg-card hover:bg-accent cursor-pointer rounded border border-solid px-4 py-1 text-[13px] disabled:cursor-not-allowed disabled:opacity-50"
        >
          Start
        </button>
        <button
          @click="resetStream"
          data-slot="dev-button"
          class="border-border bg-card hover:bg-accent cursor-pointer rounded border border-solid px-4 py-1 text-[13px] disabled:cursor-not-allowed disabled:opacity-50"
        >
          Reset
        </button>
        <label class="flex items-center gap-1.5 text-[13px]">
          Speed:
          <input v-model.number="streamSpeed" data-slot="dev-range" type="range" min="10" max="200" class="w-[120px]" />
          {{ streamSpeed }}ms
        </label>
      </div>
      <div class="my-3">
        <div
          ref="streamContainer"
          class="markdown-content chat-markdown-typography chat-citation-pills border-border bg-card rounded-md border border-solid px-3 py-2"
          v-html="streamHtml"
        ></div>
      </div>
    </section>

    <!-- Streaming Shimmer (in-progress step titles) -->
    <section class="border-border mb-9 border-b border-solid pb-6">
      <h2 class="mb-3 text-[18px] font-semibold">Streaming Shimmer</h2>
      <p class="text-muted-foreground mb-2 text-[13px]">
        The "light sweep" applied to in-progress step titles in RagPipelineProgress. Running steps shimmer; finished
        ones are static.
      </p>
      <div class="my-3 flex flex-col gap-3.5">
        <div class="action-card action-pending bg-transparent">
          <div>
            <span class="action-name text-muted-foreground text-[14px] leading-[1.55]">正在检索知识库…</span>
          </div>
        </div>
        <div class="action-card action-pending bg-transparent">
          <div>
            <span class="action-name text-muted-foreground text-[14px] leading-[1.55]">正在生成回答…</span>
          </div>
        </div>
        <div class="action-card bg-transparent">
          <div>
            <span class="action-name is-done text-muted-foreground text-[14px] leading-[1.55]"
              >检索完成（静态对照）</span
            >
          </div>
        </div>
      </div>
    </section>

    <!-- Custom Editor -->
    <section class="border-border mb-9 border-b border-solid pb-6">
      <h2 class="mb-3 text-[18px] font-semibold">Custom Input</h2>
      <p class="text-muted-foreground mb-2 text-[13px]">Paste any markdown here to test rendering.</p>
      <textarea
        v-model="customInput"
        data-slot="dev-textarea"
        class="border-border mb-3 box-border w-full resize-y rounded-md border border-solid p-2.5 [font-family:var(--app-font-family-mono)] text-[13px]"
        rows="8"
        placeholder="Type or paste markdown here..."
      ></textarea>
      <div v-if="customInput.trim()" class="my-3">
        <div
          ref="customContainer"
          class="markdown-content chat-markdown-typography chat-citation-pills border-border bg-card rounded-md border border-solid px-3 py-2"
          v-html="customHtml"
        ></div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from "vue";
import "katex/dist/katex.min.css";
// The same shared chat sheets botmsg loads, so this page renders answers with
// exactly the runtime chat styles and stays a regression target for them:
// visual changes belong in these files, not here.
import "@/components/css/chat-markdown.css";
import "@/components/css/chat-citations.css";
import "@/components/css/chat-message-shared.css";
import "@/components/css/chat-timeline-loading.css";
import { sanitizeMarkdownHTML } from "@/utils/security";
import { createChatMarkdownRenderer, renderChatMarkdown } from "@/utils/chatMarkdownRenderer";
import {
  ensureMermaidInitialized,
  enhanceMarkdownContainer,
  renderMermaidToSvg,
  createMermaidCodeRenderer,
} from "@/utils/mermaidShared";
import {
  replaceIncompleteMermaidWithPlaceholder,
  prepareStreamingMermaidMarkdown,
  extractFirstMermaidCode,
  injectCachedMermaidSvg,
} from "@/utils/chatMessageShared";

ensureMermaidInitialized();

const mermaidRenderer = createChatMarkdownRenderer({
  codeRenderer: createMermaidCodeRenderer("mermaid-test"),
});

const mermaidContainer = ref<HTMLElement | null>(null);
const streamContainer = ref<HTMLElement | null>(null);
const customContainer = ref<HTMLElement | null>(null);

const render = (raw: string): string => {
  if (!raw) return "";
  return renderChatMarkdown(raw, {
    renderer: mermaidRenderer,
    escapeMarkdown: (markdown) => markdown,
    sanitizeHtml: sanitizeMarkdownHTML,
    prepareMarkdown: (markdown) => replaceIncompleteMermaidWithPlaceholder(markdown),
  });
};

const renderStreamMarkdown = (raw: string): string => {
  if (!raw) return "";
  return renderChatMarkdown(raw, {
    renderer: mermaidRenderer,
    escapeMarkdown: (markdown) => markdown,
    sanitizeHtml: sanitizeMarkdownHTML,
    // Match production: the live chat marks unfinished answers as streaming so
    // mid-stream guards (dangling emphasis, trailing rules) are exercised here.
    streaming: isStreaming.value,
    cachedMermaidSvgHtml: streamMermaidSvgHtml.value,
    prepareMarkdown: prepareStreamingMermaidMarkdown,
    injectCachedMermaidSvg,
  });
};

// --- Test Data ---

const basicTextSample = `这是一段普通文本，包含 **加粗**、*斜体*、***加粗斜体***、~~删除线~~、行内 \`code\`。

也可以包含快捷键样式：<kbd>⌘</kbd> + <kbd>K</kbd>。

这里有一个链接：[GitHub](https://github.com)。

> 一级引用
>
> > 嵌套引用，用于对比 GPT 的引用层级与字重。

- [ ] 未完成任务
- [x] 已完成任务
`;

const latexCases = [
  { raw: "Inline math: $E = mc^2$ in the middle of text." },
  { raw: "Block math:\n$$\\int_0^\\infty e^{-x}\\,dx = 1$$" },
  { raw: "Chemical formula: $\\mathrm{Mg}^{2+} + 2\\mathrm{OH}^{-} = \\mathrm{Mg(OH)}_{2}\\downarrow$" },
  { raw: "Chemical block:\n$$\\mathrm{Cu}^{2+} + 2\\mathrm{OH}^{-} \\rightarrow \\mathrm{Cu(OH)_2}\\downarrow$$" },
  { raw: "Summation: $\\sum_{i=1}^{n} i = \\frac{n(n+1)}{2}$" },
  { raw: "Matrix:\n$$\\begin{pmatrix} a & b \\\\ c & d \\end{pmatrix}$$" },
  { raw: "Escaped delimiters: \\(\\alpha + \\beta = \\gamma\\) and \\[\\int_a^b f(x)\\,dx\\]" },
].map((tc) => ({ ...tc, html: "" }));

const codeBlockSample = `Here is some Python:

\`\`\`python
def fibonacci(n: int) -> int:
    """Calculate the nth Fibonacci number."""
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)

print(fibonacci(10))  # 55
\`\`\`

And inline code: \`const x = 42;\`
`;

const tableSample = `| Element | Symbol | Atomic Number |
|---------|--------|:-------------:|
| Hydrogen | H | 1 |
| Helium | He | 2 |
| Lithium | Li | 3 |
| Carbon | C | 6 |
`;

const listsSample = `### Ordered List
1. First item
2. Second item
   1. Nested item A
   2. Nested item B
3. Third item

### Unordered List
- Alpha
- Beta
  - Sub-item
  - Another sub-item
- Gamma

### Blockquote
> This is a blockquote with **bold** and *italic* text.
>
> It can span multiple paragraphs.
`;

const mixedSample = `## Quadratic Formula

The solutions to $ax^2 + bx + c = 0$ are given by:

$$x = \\frac{-b \\pm \\sqrt{b^2 - 4ac}}{2a}$$

### Example in Python

\`\`\`python
import math

def solve_quadratic(a, b, c):
    discriminant = b**2 - 4*a*c
    if discriminant < 0:
        return None
    x1 = (-b + math.sqrt(discriminant)) / (2*a)
    x2 = (-b - math.sqrt(discriminant)) / (2*a)
    return x1, x2
\`\`\`

| a | b | c | Solutions |
|---|---|---|-----------|
| 1 | -3 | 2 | $x = 1, 2$ |
| 1 | 0 | -4 | $x = \\pm 2$ |
| 1 | 2 | 5 | No real solutions |

> **Note:** The discriminant $\\Delta = b^2 - 4ac$ determines the nature of the roots.
`;

const mermaidSample = `\`\`\`mermaid
graph TD
    A[Start] --> B{Decision}
    B -->|Yes| C[Process A]
    B -->|No| D[Process B]
    C --> E[End]
    D --> E
\`\`\`
`;

// --- Streaming Simulation ---
const fullStreamText = `
好的，以下是根据知识库中**《xxx》学程手册**整理的有关XXX的介绍：

**XBRL（eXtensible Business Reporting Language，可扩展商业报告语言）**是一种基于XML的标准化标记语言，专门用于电子化商业和财务报告的编制、交换和分析

该数据集包含90个文本和方程对，挑战模型提取、解释和推理相互关联的财务术语和公式的能力，例如：
\`\`\`
APR = ((Fees + Interest) / Principal) × (365 / Days in Loan Term)
\`\`\`
<kb doc="2502.08127v1.pdf" chunk_id="1ecdce8a-f922-4d0c-b124-257ab4634da2" />

### 重要性


1. **AAAAA**
   - **BBBBB**
   - **CCCCC**

1. **AAA**
2. **BBB**
3. **CCC**
4. **DDD**
5. **EEE**
6. **FFF**
7. **GGG**
8. **HHH**
9. **III**

**标题：JJJ**

**标题：KKK**


The energy-mass equivalence is $E = mc^2$.

In chemistry, the neutralization reaction:

$$\\mathrm{Mg}^{2+} + 2\\mathrm{OH}^{-} = \\mathrm{Mg(OH)}_2\\downarrow$$

Here is a code example:

\`\`\`python
def greet(name):
    print(f"Hello, {name}!")
\`\`\`

And the derivative rule: $\\frac{d}{dx}\\sin x = \\cos x$.

\`\`\`mermaid
graph TD
    A[Start] --> B{Decision}
    B -->|Yes| C[Process A]
    B -->|No| D[Process B]
    C --> E[End]
    D --> E
\`\`\`

### Ordered List
1. First item
2. Second item
   1. Nested item A
   2. Nested item B
3. Third item

### Unordered List
- Alpha
- Beta
  - Sub-item
  - Another sub-item
- Gamma

### Blockquote
> This is a blockquote with **bold** and *italic* text.
>
> It can span multiple paragraphs.

\`\`\`mermaid
sequenceDiagram
    participant Driver as 驾驶员 (Driver)
    participant HMI as 人机交互界面 (HMI/Cluster)
    participant ADAS as 组合驾驶辅助系统 (ADAS ECU)
    participant Sensor as 传感器/定位模块
    participant Cloud as 云端服务平台 (可选)

    Note over ADAS, Sensor: 阶段1：正常运行与监控
    Driver->>Sensor: 车辆正常行驶中
    Sensor->>ADAS: 状态数据 (环境、定位、车辆状态)
    ADAS-->>Driver: 维持组合驾驶辅助状态 (显示图标正常)

    Note over ADAS, Sensor: 阶段2：触发接管条件
    alt 系统检测到需接管场景
        Sensor->>ADAS: 检测条件满足 (如: 地图数据缺失/限速变化/系统故障/驾驶员分心)
        ADAS->>HMI: 发送接管请求信号 (HOR Signal)
        
        Note right of HMI: 阶段3：分级提醒策略 (符合国标要求)
        HMI-->>Driver: 视觉提示 (仪表盘图标闪烁/颜色变化)
        HMI-->>Driver: 听觉提示 (轻柔蜂鸣声)
        
        ADAS->>HMI: 增强提醒 (若驾驶员无响应)
        HMI-->>Driver: 强视觉警告 (红色边框/文字)
        HMI-->>Driver: 强听觉警告 (连续急促蜂鸣)
        HMI-->>Driver: 触觉提示 (方向盘震动/座椅振动)
    end

    Note over Driver, ADAS: 阶段4：驾驶员响应处理
    alt 驾驶员及时接管
        Driver->>HMI: 手握方向盘动作 (Torque/Grip Detection)
        HMI->>ADAS: 确认驾驶员介入信号
        ADAS-->>Driver: 退出自动驾驶，切换至人工驾驶模式
        ADAS->>HMI: 清除警告提示
    else 驾驶员未响应
        alt 达到最后接管时限 (e.g., T+5s)
            ADAS->>ADAS: 启动最小风险策略 (MRM/MLR)
            ADAS->>HMI: 触发紧急减速/停车提示
            HMI-->>Driver: 紧急警告 (最高级别)
            ADAS->>Sensor: 执行安全停车动作 (靠边、刹车、双闪)
        end
    end

    Note over Cloud, Driver: 阶段5：数据记录与上报
    ADAS->>Cloud: 上传接管事件数据 (时间、原因、驾驶员响应)
    Note right of Cloud: 用于事故定责与算法优化
\`\`\`

Done.`;

const streamBuffer = ref("");
const isStreaming = ref(false);
const streamSpeed = ref(30);
const customInput = ref("");
const streamMermaidSvgHtml = ref("");
let streamTimer: ReturnType<typeof setInterval> | null = null;
let streamMermaidRenderId = 0;
let streamMermaidRenderTask: Promise<void> | null = null;

// Pre-render static fixtures once so streaming ticks do not reset other sections.
latexCases.forEach((tc) => {
  tc.html = render(tc.raw);
});
const codeBlockHtml = render(codeBlockSample);
const basicTextHtml = render(basicTextSample);
const tableHtml = render(tableSample);
const listsHtml = render(listsSample);
const mixedHtml = render(mixedSample);
const mermaidHtml = render(mermaidSample);

const streamHtml = computed(() => renderStreamMarkdown(streamBuffer.value));
const customHtml = computed(() => render(customInput.value));

const cacheStreamMermaidSvg = async () => {
  if (streamMermaidSvgHtml.value) return;

  const code = extractFirstMermaidCode(streamBuffer.value);
  if (!code) return;

  if (!streamMermaidRenderTask) {
    streamMermaidRenderTask = (async () => {
      const svg = await renderMermaidToSvg(code, `mermaid-stream-${++streamMermaidRenderId}`);
      if (svg) streamMermaidSvgHtml.value = svg;
    })().finally(() => {
      streamMermaidRenderTask = null;
    });
  }

  await streamMermaidRenderTask;
};

const startStream = () => {
  resetStream();
  isStreaming.value = true;
  let idx = 0;
  const tick = () => {
    if (idx >= fullStreamText.length) {
      if (streamTimer) clearInterval(streamTimer);
      isStreaming.value = false;
      return;
    }
    streamBuffer.value += fullStreamText[idx++];
  };
  streamTimer = setInterval(tick, streamSpeed.value);
};

const resetStream = () => {
  if (streamTimer) clearInterval(streamTimer);
  streamBuffer.value = "";
  streamMermaidSvgHtml.value = "";
  streamMermaidRenderTask = null;
  isStreaming.value = false;
};

const refreshMermaid = async (root?: HTMLElement | null) => {
  await nextTick();
  await enhanceMarkdownContainer(root ?? mermaidContainer.value);
};

onMounted(() => refreshMermaid());

watch(streamBuffer, () => {
  void cacheStreamMermaidSvg();
});

watch(streamHtml, () => {
  nextTick(() => refreshMermaid(streamContainer.value));
});

watch(isStreaming, (streaming) => {
  if (!streaming) {
    void cacheStreamMermaidSvg().then(() => refreshMermaid(streamContainer.value));
  }
});

let customMermaidTimer: ReturnType<typeof setTimeout> | null = null;
const COMPLETE_MERMAID_RE = /```mermaid[\s\S]*?```/;

watch(customInput, () => {
  if (!COMPLETE_MERMAID_RE.test(customInput.value)) return;
  if (customMermaidTimer) clearTimeout(customMermaidTimer);
  customMermaidTimer = setTimeout(() => {
    customMermaidTimer = null;
    void refreshMermaid(customContainer.value);
  }, 200);
});
</script>
