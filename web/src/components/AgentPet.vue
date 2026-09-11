<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";

import petSprite from "@/assets/pet-8v.webp";

/**
 * 8V 的精灵。
 *
 * 雪碧图 768 × 936，8 列 × 9 行，单帧 96 × 104，每行一组动作。
 * 用像素定位而不是百分比：百分比的 x% 对应第 (列数-1)·x/100 帧，
 * steps() 均分 0%→100% 会落在两帧之间，画面就会错位。
 */
export type PetState =
  | "idle"        // 待机
  | "roaming"     // 溜达（长时间没人说话）
  | "thinking"    // 等模型开口
  | "searching"   // 正在调工具
  | "speaking"    // 正在吐字
  | "happy"       // 答完了 / 被摸了
  | "sad"         // 出错
  | "notice";     // 举手提醒

const frameWidth = 96;
const frameHeight = 104;
const petColumns = 8;
const petRowCount = 9;

// 行索引从 0 起。notice 复用第 3 行的举手姿势——它本来就是在说话，
// 只是说的是提醒；第 7 行（疾冲）当提醒用太跳脱，暂时留空。
const petRows: Record<PetState, number> = {
  idle: 0,
  roaming: 1,
  speaking: 3,
  happy: 4,
  sad: 5,
  thinking: 6,
  notice: 3,
  searching: 8,
};

const petDurations: Record<PetState, string> = {
  idle: "2.4s",
  roaming: "0.9s",
  speaking: "0.7s",
  happy: "0.8s",
  sad: "1.6s",
  thinking: "1.2s",
  notice: "1.4s",
  searching: "0.8s",
};

const props = withDefaults(
  defineProps<{
    state?: PetState;
    message?: string;
    /** 初始位置；未提供时贴右下角。拖动后由父层持久化。 */
    position?: { x: number; y: number } | null;
    draggable?: boolean;
  }>(),
  { state: "idle", message: "", position: null, draggable: true },
);

const emit = defineEmits<{
  (e: "move", position: { x: number; y: number }): void;
  (e: "poke"): void;
  /** 一被抓住就发出：上层据此掐掉正在显示的话，别让气泡跟着跑。 */
  (e: "grab"): void;
}>();

const rootElement = ref<HTMLElement | null>(null);
const dragging = ref(false);
const hovered = ref(false);

// 气泡的最大宽度，同时用于判断左边放不放得下。与 CSS 里的 max-width 一致。
const bubbleMaxWidth = 210;
const bubbleGap = 10;

// 视口尺寸要是响应式的，否则靠边判断在窗口缩放后就失效了。
const viewportWidth = ref(typeof window === "undefined" ? 1280 : window.innerWidth);

// 朝向。素材里奔跑那一行画的是**朝右**跑，所以朝左才需要水平翻转。
const facing = ref<"left" | "right">("left");

// 拖动时强制播放奔跑动画，与外部传入的状态无关。
const effectiveState = computed<PetState>(() => (dragging.value ? "roaming" : props.state));

// 只有奔跑才有方向可言。其余动作保持原朝向——否则查资料那一行的放大镜
// 会翻到另一只手上，耷拉的耳朵也会左右颠倒。
const flip = computed(() => {
  if (effectiveState.value !== "roaming") return "1";

  return facing.value === "left" ? "-1" : "1";
});

// 当前落点。null 表示还没拖过，用 CSS 的默认右下角定位。
const placement = ref<{ x: number; y: number } | null>(props.position);

const petMargin = 12;

const spriteStyle = computed(() => ({
  backgroundImage: `url(${petSprite})`,
  backgroundPositionY: `${-petRows[effectiveState.value] * frameHeight}px`,
  // 拖着跑的时候步频快一点
  animationDuration: dragging.value ? "0.5s" : petDurations[effectiveState.value],
  "--pet-strip": `${-petColumns * frameWidth}px`,
  "--pet-flip": flip.value,
}));

const rootStyle = computed(() => {
  if (!placement.value) return undefined;

  return { left: `${placement.value.x}px`, top: `${placement.value.y}px`, right: "auto", bottom: "auto" };
});

/** 精灵左上角在视口中的位置。没拖过时就是 CSS 的默认右下角。 */
const spriteLeft = computed(() =>
  placement.value ? placement.value.x : viewportWidth.value - petMargin - 8 - frameWidth,
);

const spriteTop = computed(() => placement.value?.y ?? Number.POSITIVE_INFINITY);

/** 精灵两侧各自剩多少横向空间。 */
const sideRoom = computed(() => ({
  left: spriteLeft.value - petMargin - bubbleGap,
  right: viewportWidth.value - (spriteLeft.value + frameWidth) - petMargin - bubbleGap,
}));

/**
 * 气泡默认在精灵左边；左边塞不下就翻到右边。
 *
 * 翻转的是气泡自己的绝对定位，精灵不会因此移动——气泡已经脱离文档流。
 * 用"哪边更宽"而不是固定阈值：窄屏上两边都不够时，固定阈值会把气泡翻到
 * 更挤的一侧，反而溢出得更厉害。
 */
const bubbleOnRight = computed(() => {
  if (sideRoom.value.left >= bubbleMaxWidth) return false;

  return sideRoom.value.right > sideRoom.value.left;
});

/** 把 max-width 钳到所选一侧的实际空间，保证不会溢出视口。 */
const bubbleStyle = computed(() => {
  const room = bubbleOnRight.value ? sideRoom.value.right : sideRoom.value.left;

  return { maxWidth: `${Math.max(120, Math.min(bubbleMaxWidth, room))}px` };
});

/** 贴近顶部时气泡改为向下展开，否则会顶出视口。 */
const bubbleBelow = computed(() => spriteTop.value < 150);

/** 把落点夹回视口内，窗口缩小后精灵才不会跑到看不见的地方。 */
function clamp(x: number, y: number) {
  const maxX = window.innerWidth - frameWidth - petMargin;
  const maxY = window.innerHeight - frameHeight - petMargin;

  return {
    x: Math.min(Math.max(x, petMargin), Math.max(maxX, petMargin)),
    y: Math.min(Math.max(y, petMargin), Math.max(maxY, petMargin)),
  };
}

let pointerId: number | null = null;
let grabOffset = { x: 0, y: 0 };
let origin = { x: 0, y: 0 };
let moved = false;

// 判定为"拖动"而不是"戳一下"的最小位移。
// 不能用 movement > 0：按下时手指/鼠标的亚像素抖动就会被当成拖动，
// 结果就是点它几乎永远没反应。
const dragThreshold = 4;

// 朝向的死区：小于这个位移不改朝向，免得抓着不动时左右乱翻。
const facingDeadzone = 2;

function onPointerDown(event: PointerEvent) {
  if (!props.draggable || event.button !== 0) return;

  const rect = rootElement.value?.getBoundingClientRect();
  if (!rect) return;

  pointerId = event.pointerId;
  moved = false;
  origin = { x: event.clientX, y: event.clientY };
  grabOffset = { x: event.clientX - rect.left, y: event.clientY - rect.top };

  // 抓起来默认朝左跑
  facing.value = "left";

  // 捕获指针：拖到精灵外面也不会丢失事件。
  (event.target as Element).setPointerCapture?.(event.pointerId);
  dragging.value = true;

  // 一抓住就立刻停止说话：否则气泡会跟着精灵满屏跑。
  emit("grab");

  event.preventDefault();
}

function onPointerMove(event: PointerEvent) {
  if (!dragging.value || event.pointerId !== pointerId) return;

  const dx = event.clientX - origin.x;
  const dy = event.clientY - origin.y;

  if (Math.hypot(dx, dy) > dragThreshold) moved = true;

  // 朝向跟着本帧的移动方向走，停下时保持上一次的朝向。
  if (event.movementX < -facingDeadzone) facing.value = "left";
  else if (event.movementX > facingDeadzone) facing.value = "right";

  placement.value = clamp(event.clientX - grabOffset.x, event.clientY - grabOffset.y);
}

function onPointerUp(event: PointerEvent) {
  if (event.pointerId !== pointerId) return;

  dragging.value = false;
  pointerId = null;
  facing.value = "left";

  if (moved && placement.value) {
    emit("move", placement.value);
  } else {
    // 没挪动 = 戳了一下
    emit("poke");
  }
}

function handleResize() {
  viewportWidth.value = window.innerWidth;

  if (!placement.value) return;

  placement.value = clamp(placement.value.x, placement.value.y);
}

onMounted(() => {
  if (placement.value) placement.value = clamp(placement.value.x, placement.value.y);
  window.addEventListener("resize", handleResize);
});

onBeforeUnmount(() => window.removeEventListener("resize", handleResize));
</script>

<template>
  <div
    ref="rootElement"
    class="agent-pet"
    :class="{ 'is-dragging': dragging, 'has-placement': !!placement }"
    :style="rootStyle"
  >
    <!-- 气泡绝对定位，不占布局空间：它必须不能把精灵挤走。
         永远不接收指针事件；拖动期间一律不显示，即便上层忘了清空。 -->
    <transition name="bubble">
      <p
        v-if="message && !dragging"
        class="pet-bubble"
        :class="{ 'is-right': bubbleOnRight, 'is-below': bubbleBelow }"
        :style="bubbleStyle"
      >
        <span class="pet-bubble-name">8V</span>
        {{ message }}
      </p>
    </transition>

    <!--
      只有精灵本体可交互（抓取 + 悬浮），其余区域指针穿透。
      role/aria-label 让它至少能被识别，但它是纯装饰，不承载任何必要功能。
    -->
    <div
      class="pet-sprite"
      :class="[`is-${effectiveState}`, { 'is-hovered': hovered && !dragging }]"
      :style="spriteStyle"
      role="img"
      aria-label="8V 精灵"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      @pointerenter="hovered = true"
      @pointerleave="hovered = false"
    />
  </div>
</template>

<style scoped>
/* 容器尺寸 = 精灵尺寸。气泡是绝对定位的子元素，不撑大容器，
   所以无论气泡多长、在哪一侧，精灵的位置都不会被推动。 */
.agent-pet {
  position: fixed;
  right: 20px;
  bottom: 20px;
  z-index: 60;
  width: 96px;
  height: 104px;
  /* 容器不接收事件，只有 .pet-sprite 单独打开 */
  pointer-events: none;
  user-select: none;
}

.agent-pet.has-placement {
  /* 拖动后改用 left/top 定位 */
  right: auto;
  bottom: auto;
}

.pet-sprite {
  flex: 0 0 auto;
  width: 96px;
  height: 104px;
  background-size: 768px 936px;
  background-repeat: no-repeat;
  transform-origin: bottom center;
  /* 只有本体可抓 */
  pointer-events: auto;
  cursor: grab;
  touch-action: none;
  /* --pet-flip 由脚本给：朝右时为 -1，水平镜像。
     所有 transform 都要乘上它，否则一翻转就丢失缩放。 */
  --pet-flip: 1;
  transform: scaleX(var(--pet-flip));
  transition: transform 0.18s ease, filter 0.18s ease;
  animation-name: pet-frames;
  /* steps(8)：0 → -96 → … → -672，正好 8 个整帧 */
  animation-timing-function: steps(8);
  animation-iteration-count: infinite;
}

@keyframes pet-frames {
  from { background-position-x: 0; }
  to   { background-position-x: var(--pet-strip); }
}

/* 悬浮：微微放大并上浮，给一点"它注意到你了"的反馈 */
.pet-sprite.is-hovered {
  transform: translateY(-4px) scaleX(calc(var(--pet-flip) * 1.08)) scaleY(1.08);
  filter: drop-shadow(0 4px 6px rgba(150, 120, 70, 0.35));
}

/* 拖动时抬起来一点，配合奔跑动画。
   不加 transition：拖动中每帧都在改位置，过渡会让它拖泥带水。 */
.is-dragging .pet-sprite {
  cursor: grabbing;
  transform: translateY(-6px) scaleX(calc(var(--pet-flip) * 1.12)) scaleY(1.12);
  filter: drop-shadow(0 9px 10px rgba(150, 120, 70, 0.42));
  transition: none;
}

/* 得意只蹦两次就回待机，一直跳很吵 */
.pet-sprite.is-happy { animation-iteration-count: 2; }

/* 沿用 .pixel-panel 的做法：木色描边 + 内嵌一圈奶油色高光 + 硬投影，
   这样气泡和侧栏、面板是同一套视觉语言，而不是另起一种风格。 */
.pet-bubble {
  position: absolute;
  /* 默认贴在精灵左侧、略高于底部 */
  right: calc(100% + 10px);
  bottom: 28px;
  /* max-content + max-width：短句不换行，长句才折到 210px，
     不会出现被挤成一列竖排字的情况 */
  width: max-content;
  max-width: 210px;
  margin: 0;
  padding: 10px 13px 11px;
  border: 2px solid var(--wood-500, #895033);
  border-radius: 10px;
  color: var(--wood-700, #533225);
  background:
    linear-gradient(rgba(255, 255, 255, 0.42), rgba(255, 255, 255, 0)) padding-box,
    var(--cream-50, #fffaf0);
  box-shadow:
    inset 0 0 0 3px var(--cream-100, #fff2cf),
    var(--shadow-pixel, 0 4px 0 rgba(72, 44, 28, 0.18));
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

/* 说话人标签，让气泡不是一块无主的文字 */
.pet-bubble-name {
  display: block;
  margin-bottom: 3px;
  color: var(--wood-400, #a9683f);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

/* 左边放不下时整体翻到右侧。翻的是气泡，精灵纹丝不动。 */
.pet-bubble.is-right {
  right: auto;
  left: calc(100% + 10px);
}

/* 贴近顶部时改为向下展开，否则气泡会顶出视口上沿 */
.pet-bubble.is-below {
  bottom: auto;
  top: 12px;
}

/* 尾巴用两层：外层木色描边，内层奶油色盖住边框，拼出一个真正的尖角 */
.pet-bubble::before,
.pet-bubble::after {
  content: "";
  position: absolute;
  right: -10px;
  bottom: 12px;
  width: 0;
  height: 0;
  border-top: 7px solid transparent;
  border-bottom: 7px solid transparent;
  border-left: 10px solid var(--wood-500, #895033);
}

/* 内层比外层矮 2px，所以 bottom 要 +1 才和外层同心，
   否则尖角上沿会露出一条不对称的木色。 */
.pet-bubble::after {
  right: -6px;
  bottom: 13px;
  border-top-width: 6px;
  border-bottom-width: 6px;
  border-left: 8px solid var(--cream-100, #fff2cf);
}

/* 气泡在右侧时，尾巴要翻到左边并改为朝左指 */
.pet-bubble.is-right::before,
.pet-bubble.is-right::after {
  right: auto;
  border-left: none;
}

.pet-bubble.is-right::before {
  left: -10px;
  border-right: 10px solid var(--wood-500, #895033);
}

.pet-bubble.is-right::after {
  left: -6px;
  border-right: 8px solid var(--cream-100, #fff2cf);
}

/* 向下展开时尾巴贴顶部，指向下方的精灵 */
.pet-bubble.is-below::before { bottom: auto; top: 12px; }
.pet-bubble.is-below::after { bottom: auto; top: 13px; }

/* 冒出来时轻微弹一下，像是"啵"地出现 */
.bubble-enter-active { transition: opacity 0.2s ease, transform 0.26s cubic-bezier(0.34, 1.56, 0.64, 1); }
.bubble-leave-active { transition: opacity 0.16s ease, transform 0.16s ease; }
.bubble-enter-from { opacity: 0; transform: translateY(6px) scale(0.9); }
.bubble-leave-to { opacity: 0; transform: translateY(3px) scale(0.97); }

/* 窄屏留精灵本体。用 transform 缩放而不是改宽高——
   改了宽高，像素定位的帧就对不齐了。 */
/* max-width 由脚本按实际空间下发，这里只调字号 */
@media (max-width: 900px) {
  .pet-bubble { font-size: 11px; }
  .pet-sprite { transform: scaleX(calc(var(--pet-flip) * 0.72)) scaleY(0.72); }
  .pet-sprite.is-hovered {
    transform: translateY(-3px) scaleX(calc(var(--pet-flip) * 0.78)) scaleY(0.78);
  }
  .is-dragging .pet-sprite {
    transform: translateY(-4px) scaleX(calc(var(--pet-flip) * 0.8)) scaleY(0.8);
  }
}

/* 减少动效时保留翻转（那是朝向信息，不是装饰），只停掉逐帧动画 */
@media (prefers-reduced-motion: reduce) {
  .pet-sprite {
    animation: none;
    background-position-x: 0;
    transition: none;
  }
  .bubble-enter-active,
  .bubble-leave-active { transition: none; }
}
</style>
