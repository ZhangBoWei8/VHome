<script setup lang="ts">
import {
  CalendarClock, ChevronLeft, ChevronRight, Clock3, Edit3, ListChecks,
  Minus, Plus, RefreshCw, Search, ShieldOff, Trash2, Users, X,
} from "@lucide/vue";
import { computed, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";

import {
  createMemo, deleteMemo, dismissMemo, getCreatedMemos, getMemo,
  getMyMemoCalendar, getMyMemos, listMemoMemberOptions, searchMyMemos,
  syncHolidayCalendar, updateMemo,
  type CalendarDayOverride, type Memo, type MemoCalendar, type MemoMemberOption,
} from "@/api";
import { useSessionStore } from "@/stores/session";

const route = useRoute();
const session = useSessionStore();
const now = new Date();
const visibleMonth = ref(new Date(now.getFullYear(), now.getMonth(), 1));
const selectedDate = ref(formatDateKey(now));
const calendar = ref<MemoCalendar>({ month: "", memos: [], calendar_overrides: [] });
const members = ref<MemoMemberOption[]>([]);
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const success = ref("");
const editorOpen = ref(false);
const editing = ref<Memo | null>(null);
const detailMemo = ref<Memo | null>(null);
const listOpen = ref(false);
const listTab = ref<"received" | "created">("received");
const listItems = ref<Memo[]>([]);
const searchOpen = ref(false);
const searchKeyword = ref("");
const searchResults = ref<Memo[]>([]);
const syncing = ref(false);
const form = ref({ title: "", description: "", remindAt: "", recipientIDs: [] as number[] });
const lunarFormatter = new Intl.DateTimeFormat("zh-CN-u-ca-chinese", { dateStyle: "full" });

const monthKey = computed(() => `${visibleMonth.value.getFullYear()}-${String(visibleMonth.value.getMonth() + 1).padStart(2, "0")}`);
const monthTitle = computed(() => `${visibleMonth.value.getFullYear()} 年 ${visibleMonth.value.getMonth() + 1} 月`);
const memberMap = computed(() => new Map(members.value.map((member) => [member.id, member.display_name])));
const overrideMap = computed(() => new Map(calendar.value.calendar_overrides.map((item) => [item.calendar_date.slice(0, 10), item])));
const selectedMemos = computed(() => calendar.value.memos.filter((memo) => dateKeyFromISO(memo.remind_at) === selectedDate.value));
const hourlyMemos = computed(() => {
  const grouped = new Map<number, Memo[]>();
  for (const memo of selectedMemos.value) {
    const hour = new Date(memo.remind_at).getHours();
    grouped.set(hour, [...(grouped.get(hour) ?? []), memo]);
  }
  return grouped;
});

interface CalendarCell { date: Date; key: string; inMonth: boolean; override?: CalendarDayOverride; memos: Memo[] }
const calendarCells = computed<CalendarCell[]>(() => {
  const first = visibleMonth.value;
  const mondayOffset = (first.getDay() + 6) % 7;
  const start = new Date(first.getFullYear(), first.getMonth(), 1 - mondayOffset);
  return Array.from({ length: 42 }, (_, index) => {
    const date = new Date(start.getFullYear(), start.getMonth(), start.getDate() + index);
    const key = formatDateKey(date);
    return {
      date, key, inMonth: date.getMonth() === first.getMonth(),
      override: overrideMap.value.get(key),
      memos: calendar.value.memos.filter((memo) => dateKeyFromISO(memo.remind_at) === key),
    };
  });
});

function formatDateKey(date: Date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}
function dateKeyFromISO(value: string) {
  const date = new Date(value);
  return formatDateKey(date);
}
function isToday(key: string) { return key === formatDateKey(new Date()); }
function isRestDay(cell: CalendarCell) {
  if (cell.override?.day_type === "TRANSFER_WORKDAY") return false;
  return cell.override?.day_type === "HOLIDAY" || cell.date.getDay() === 0 || cell.date.getDay() === 6;
}
function lunarDay(date: Date) {
  const match = lunarFormatter.format(date).match(/年(.+?)月(.+?)星期/);
  if (!match) return "";
  return match[2] === "初一" ? `${match[1]}月` : match[2];
}
function formatTime(value: string) {
  return new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }).format(new Date(value));
}
function avatar(value: string) {
  return ({ man: "👨", woman: "👩", boy: "👦", girl: "👧", dog: "🐶" } as Record<string, string>)[value] ?? "🌻";
}
function recipientText(memo: Memo) {
  return memo.recipient_ids.map((id) => memberMap.value.get(id) ?? `成员 ${id}`).join("、");
}
function nextHalfHour(dateKey = selectedDate.value) {
  const current = new Date();
  let candidate = new Date(`${dateKey}T09:00:00`);
  if (formatDateKey(current) === dateKey) {
    candidate = new Date(current);
    candidate.setSeconds(0, 0);
    candidate.setMinutes(candidate.getMinutes() < 30 ? 30 : 60);
  }
  if (candidate <= current) {
    candidate = new Date(current.getTime() + 30 * 60_000);
    candidate.setSeconds(0, 0);
    candidate.setMinutes(candidate.getMinutes() < 30 ? 30 : 60);
  }
  return `${formatDateKey(candidate)}T${String(candidate.getHours()).padStart(2, "0")}:${String(candidate.getMinutes()).padStart(2, "0")}`;
}

async function loadCalendar() {
  loading.value = true;
  error.value = "";
  try { calendar.value = await getMyMemoCalendar(monthKey.value); }
  catch (cause) { error.value = cause instanceof Error ? cause.message : "读取家庭备忘失败"; }
  finally { loading.value = false; }
}
async function loadMembers() {
  members.value = await listMemoMemberOptions();
}
function selectDay(cell: CalendarCell) {
  selectedDate.value = cell.key;
  if (!cell.inMonth) visibleMonth.value = new Date(cell.date.getFullYear(), cell.date.getMonth(), 1);
}
function changeMonth(delta: number) {
  visibleMonth.value = new Date(visibleMonth.value.getFullYear(), visibleMonth.value.getMonth() + delta, 1);
  selectedDate.value = formatDateKey(visibleMonth.value);
}
function openCreate() {
  editing.value = null;
  const currentMember = members.value.find((member) => member.is_current);
  form.value = { title: "", description: "", remindAt: nextHalfHour(), recipientIDs: currentMember ? [currentMember.id] : [] };
  editorOpen.value = true;
}
function openEdit(memo: Memo) {
  if (!memo.can_edit) { detailMemo.value = memo; return; }
  editing.value = memo;
  const date = new Date(memo.remind_at);
  const local = `${formatDateKey(date)}T${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
  form.value = { title: memo.title, description: memo.description, remindAt: local, recipientIDs: [...memo.recipient_ids] };
  detailMemo.value = null;
  editorOpen.value = true;
}
async function saveMemo() {
  if (saving.value || !form.value.recipientIDs.length) return;
  saving.value = true; error.value = ""; success.value = "";
  try {
    const input = {
      title: form.value.title.trim(), description: form.value.description.trim(),
      remind_at: form.value.remindAt, recipient_ids: form.value.recipientIDs,
      ...(editing.value ? { version: editing.value.version } : {}),
    };
    if (editing.value) await updateMemo(editing.value.id, input);
    else await createMemo(input);
    editorOpen.value = false;
    success.value = editing.value ? "备忘事项已更新" : "备忘事项已创建";
    await loadCalendar();
    window.dispatchEvent(new Event("vhome:notifications-changed"));
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "保存备忘事项失败"; }
  finally { saving.value = false; }
}
async function removeMemo(memo: Memo) {
  if (!window.confirm(`确定完全删除“${memo.title}”吗？所有收件人都将看不到它。`)) return;
  try {
    await deleteMemo(memo.id, memo.version); detailMemo.value = null;
    success.value = "备忘事项已删除"; await loadCalendar(); await refreshList();
    window.dispatchEvent(new Event("vhome:notifications-changed"));
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "删除失败"; }
}
async function muteMemo(memo: Memo) {
  try {
    await dismissMemo(memo.id, memo.version); detailMemo.value = null;
    success.value = "已屏蔽这条提醒"; await loadCalendar(); await refreshList();
    window.dispatchEvent(new Event("vhome:notifications-changed"));
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "屏蔽失败"; }
}
async function refreshList() {
  if (!listOpen.value) return;
  listItems.value = listTab.value === "received" ? await getMyMemos() : await getCreatedMemos();
}
async function openList() { listOpen.value = true; await refreshList(); }
async function runSearch() {
  if (!searchKeyword.value.trim()) return;
  try { searchResults.value = await searchMyMemos(searchKeyword.value.trim()); }
  catch (cause) { error.value = cause instanceof Error ? cause.message : "搜索失败"; }
}
async function syncCalendar() {
  syncing.value = true; error.value = "";
  try {
    const result = await syncHolidayCalendar(visibleMonth.value.getFullYear());
    success.value = `已同步 ${result.synced_days} 个国务院节假日/调休日期`;
    await loadCalendar();
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "同步日历失败"; }
  finally { syncing.value = false; }
}

watch(monthKey, loadCalendar);
watch(listTab, refreshList);
watch(() => route.query.memo, async (value) => {
  const requestedID = Number(value);
  if (requestedID > 0) detailMemo.value = await getMemo(requestedID);
});
onMounted(async () => {
  try {
    await Promise.all([loadMembers(), loadCalendar()]);
    const requestedID = Number(route.query.memo);
    if (requestedID > 0) detailMemo.value = await getMemo(requestedID);
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "初始化备忘录失败"; }
});
</script>

<template>
  <div class="page-stack memo-page">
    <section class="inventory-hero memo-hero">
      <div><p class="eyebrow">FAMILY MEMO</p><h1>家庭备忘</h1><p>把未来要做的事放进日历，在约定的时刻提醒家人。</p></div>
      <CalendarClock :size="44" />
    </section>

    <p v-if="error" class="form-message error">{{ error }}</p>
    <p v-if="success" class="form-message success">{{ success }}</p>

    <section class="pixel-panel memo-calendar-panel">
      <header class="memo-calendar-header">
        <div class="memo-month-switcher">
          <button class="icon-button" type="button" aria-label="查看上个月" @click="changeMonth(-1)"><ChevronLeft /></button>
          <h2>{{ monthTitle }}</h2>
          <button class="icon-button" type="button" aria-label="查看下个月" @click="changeMonth(1)"><ChevronRight /></button>
        </div>
        <div class="memo-toolbar">
          <button v-if="session.memberRole === 'OWNER'" class="memo-tool-button" type="button" aria-label="同步本年法定节假日" :disabled="syncing" @click="syncCalendar"><RefreshCw :class="{ spinning: syncing }" /></button>
          <button class="memo-tool-button" type="button" aria-label="查找备忘信息" @click="searchOpen = true"><Search /></button>
          <button class="memo-tool-button" type="button" aria-label="查看全部备忘事项" @click="openList"><ListChecks /></button>
          <button class="memo-tool-button danger-soft" type="button" aria-label="管理或删除备忘事项" @click="openList"><Minus /></button>
          <button class="memo-tool-button primary" type="button" aria-label="新建备忘事项" data-tooltip-placement="left" @click="openCreate"><Plus /></button>
        </div>
      </header>

      <div class="memo-weekdays"><span v-for="day in ['一','二','三','四','五','六','日']" :key="day">周{{ day }}</span></div>
      <div v-if="loading" class="memo-loading">正在翻开家庭日历…</div>
      <div v-else class="memo-calendar-grid">
        <button
          v-for="cell in calendarCells" :key="cell.key" type="button" class="memo-day"
          :class="{ muted: !cell.inMonth, selected: selectedDate === cell.key, today: isToday(cell.key), rest: isRestDay(cell) }"
          @click="selectDay(cell)"
        >
          <span class="memo-day-number">{{ cell.date.getDate() }}</span>
          <small class="memo-lunar">{{ lunarDay(cell.date) }}</small>
          <em v-if="cell.override" :class="cell.override.day_type === 'TRANSFER_WORKDAY' ? 'work-label' : 'holiday-label'">
            {{ cell.override.day_type === "TRANSFER_WORKDAY" ? "调" : cell.override.holiday_name }}
          </em>
          <span v-for="memo in cell.memos.slice(0, 2)" :key="memo.id" class="memo-day-event" @click.stop="openEdit(memo)">{{ memo.title }}</span>
          <small v-if="cell.memos.length > 2" class="memo-more">另 {{ cell.memos.length - 2 }} 项</small>
        </button>
      </div>
    </section>

    <section class="pixel-panel memo-timeline-panel">
      <header><div><h2>{{ selectedDate }} 时间表</h2><p>每小时最多显示 5 条发送给你的事项，时间区间为左闭右开。</p></div><button class="secondary-button" @click="openCreate"><Plus :size="16" /> 在这一天新建</button></header>
      <div class="memo-timeline">
        <div v-for="hour in 24" :key="hour - 1" class="memo-hour-row">
          <time>{{ String(hour - 1).padStart(2, "0") }}:00</time>
          <div class="memo-hour-line">
            <button v-for="memo in hourlyMemos.get(hour - 1) ?? []" :key="memo.id" class="memo-time-card" type="button" @click="openEdit(memo)">
              <strong>{{ new Date(memo.remind_at).toLocaleTimeString('zh-CN', {hour:'2-digit',minute:'2-digit',hour12:false}) }} · {{ memo.title }}</strong>
              <span>{{ memo.description || `提醒 ${recipientText(memo)}` }}</span>
            </button>
          </div>
        </div>
      </div>
    </section>

    <div v-if="editorOpen" class="modal-backdrop" @click.self="editorOpen = false">
      <form class="pixel-modal memo-editor material-form" @submit.prevent="saveMemo">
        <header><div><p class="eyebrow">{{ editing ? 'EDIT MEMO' : 'NEW MEMO' }}</p><h2>{{ editing ? "编辑备忘" : "新建备忘" }}</h2></div><button class="icon-button" type="button" aria-label="关闭备忘编辑窗口" data-tooltip-placement="left" @click="editorOpen = false"><X /></button></header>
        <label><span>事项标题</span><input v-model="form.title" required maxlength="128" placeholder="例如：带小狗去打疫苗" /></label>
        <label><span>提醒时间（30 分钟为单位）</span><input v-model="form.remindAt" type="datetime-local" required step="1800" /></label>
        <label><span>备忘内容（可选）</span><textarea v-model="form.description" maxlength="5000" rows="5" placeholder="补充地址、物品或其他需要记住的信息" /></label>
        <fieldset class="memo-recipient-field"><legend><Users :size="15" /> 提醒谁</legend><div><label v-for="member in members" :key="member.id" class="memo-recipient"><input v-model="form.recipientIDs" type="checkbox" :value="member.id" /><span>{{ avatar(member.avatar_key) }}</span><strong>{{ member.display_name }}</strong><small v-if="member.is_current">我</small></label></div></fieldset>
        <p v-if="!form.recipientIDs.length" class="form-help warning">请至少选择一位家庭成员。</p>
        <button class="primary-pixel-button" :disabled="saving || !form.recipientIDs.length">{{ saving ? "保存中…" : editing ? "保存修改" : "创建备忘" }}</button>
      </form>
    </div>

    <div v-if="detailMemo" class="modal-backdrop" @click.self="detailMemo = null">
      <article class="pixel-modal memo-detail">
        <header><div><p class="eyebrow">MEMO DETAIL</p><h2>{{ detailMemo.title }}</h2></div><button class="icon-button" aria-label="关闭备忘详情" data-tooltip-placement="left" @click="detailMemo = null"><X /></button></header>
        <p class="memo-detail-time"><Clock3 :size="17" /> {{ formatTime(detailMemo.remind_at) }}</p>
        <p class="memo-detail-description">{{ detailMemo.description || "没有补充内容。" }}</p>
        <p class="memo-detail-recipients"><Users :size="16" /> 提醒：{{ recipientText(detailMemo) }}</p>
        <footer>
          <button v-if="detailMemo.can_dismiss" class="secondary-button" @click="muteMemo(detailMemo)"><ShieldOff :size="16" /> 屏蔽我的提醒</button>
          <button v-if="detailMemo.can_edit" class="secondary-button" @click="openEdit(detailMemo)"><Edit3 :size="16" /> 编辑</button>
          <button v-if="detailMemo.can_delete" class="danger-button" @click="removeMemo(detailMemo)"><Trash2 :size="16" /> 删除</button>
        </footer>
      </article>
    </div>

    <div v-if="listOpen" class="modal-backdrop" @click.self="listOpen = false">
      <section class="pixel-modal memo-list-modal">
        <header><div><p class="eyebrow">MEMO LIST</p><h2>全部备忘事项</h2></div><button class="icon-button" aria-label="关闭备忘列表" data-tooltip-placement="left" @click="listOpen = false"><X /></button></header>
        <div class="memo-list-tabs"><button :class="{active:listTab==='received'}" @click="listTab='received'">提醒我的</button><button :class="{active:listTab==='created'}" @click="listTab='created'">我创建的</button></div>
        <div class="memo-list-content">
          <article v-for="memo in listItems" :key="memo.id" class="memo-list-row"><div><strong>{{ memo.title }}</strong><small>{{ formatTime(memo.remind_at) }} · {{ recipientText(memo) }}</small></div><div><button v-if="memo.can_edit" class="icon-button" aria-label="编辑这条备忘" data-tooltip-placement="top" @click="listOpen=false;openEdit(memo)"><Edit3 /></button><button v-if="memo.can_dismiss" class="icon-button" aria-label="屏蔽对我的提醒" data-tooltip-placement="top" @click="muteMemo(memo)"><ShieldOff /></button><button v-if="memo.can_delete" class="icon-button danger-soft" aria-label="完全删除这条备忘" data-tooltip-placement="top-left" @click="removeMemo(memo)"><Trash2 /></button></div></article>
          <p v-if="!listItems.length" class="notice-empty">这里还没有备忘事项。</p>
        </div>
      </section>
    </div>

    <div v-if="searchOpen" class="modal-backdrop" @click.self="searchOpen = false">
      <section class="pixel-modal memo-list-modal">
        <header><div><p class="eyebrow">SEARCH MEMO</p><h2>搜索我的备忘</h2></div><button class="icon-button" aria-label="关闭备忘搜索" data-tooltip-placement="left" @click="searchOpen = false"><X /></button></header>
        <form class="memo-search-form" @submit.prevent="runSearch"><input v-model="searchKeyword" maxlength="100" autofocus placeholder="输入标题或备忘内容" /><button class="primary-pixel-button compact"><Search :size="16" /> 搜索</button></form>
        <div class="memo-list-content"><button v-for="memo in searchResults" :key="memo.id" class="memo-search-result" @click="searchOpen=false;openEdit(memo)"><strong>{{ memo.title }}</strong><small>{{ formatTime(memo.remind_at) }}</small></button><p v-if="searchKeyword && !searchResults.length" class="notice-empty">没有找到匹配事项。</p></div>
      </section>
    </div>
  </div>
</template>
