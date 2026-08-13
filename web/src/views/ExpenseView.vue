<script setup lang="ts">
import {
  ChevronLeft,
  ChevronRight,
  Download,
  Edit3,
  Plus,
  ReceiptText,
  Trash2,
  WalletCards,
  X,
} from "@lucide/vue";
import { computed, onMounted, reactive, ref, watch } from "vue";

import {
  createExpense,
  deleteExpense,
  downloadExpenseExport,
  getCollectiveMonthlyExpenses,
  getMyMonthlyExpenses,
  listExpenseCategories,
  updateExpense,
  type ExpenseCategory,
  type ExpenseExportView,
  type ExpenseInput,
  type ExpenseRecord,
  type ExpenseScope,
  type MonthlyExpenseView,
} from "@/api";
import { useSessionStore } from "@/stores/session";

type LedgerTab = "MINE" | "COLLECTIVE";

const session = useSessionStore();
const tab = ref<LedgerTab>("MINE");
const month = ref(monthText(new Date()));
const categories = ref<ExpenseCategory[]>([]);
const ledger = ref<MonthlyExpenseView>(emptyLedger(month.value));
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const modalOpen = ref(false);
const editing = ref<ExpenseRecord | null>(null);

const form = reactive<ExpenseInput>({
  category_id: 0,
  expense_scope: "PERSONAL",
  title: "",
  amount: "",
  spent_on: todayText(),
  note: "",
});

const categoryEmoji: Record<string, string> = {
  household: "🏡",
  shopping: "🛍️",
  medical: "💊",
  transportation: "🚆",
  education: "📚",
  other: "🧾",
};
const chartColors = ["#7ea653", "#d9a13d", "#5f9fac", "#bd7462", "#9b7ab1", "#a9683f"];

const displayedMonth = computed(() => {
	const year = Number(month.value.slice(0, 4));
	const monthNumber = Number(month.value.slice(5, 7));
  return `${year} 年 ${monthNumber} 月`;
});
const isCurrentMonth = computed(() => month.value === monthText(new Date()));
const canEditOther = computed(() => session.memberRole === "OWNER" || session.memberRole === "ADMIN");
const categoryGradient = computed(() => {
  if (!ledger.value.total_amount_cents || !ledger.value.category_totals.length) return "#eadcb8";
  let cursor = 0;
  const stops = ledger.value.category_totals.map((item, index) => {
    const start = cursor;
    cursor += item.percentage;
    return `${chartColors[index % chartColors.length]} ${start}% ${cursor}%`;
  });
  return `conic-gradient(${stops.join(", ")})`;
});
const dailyTotals = computed(() => {
  const values = new Map<string, number>();
  for (const record of ledger.value.records) {
    const day = record.spent_on.slice(8, 10);
    values.set(day, (values.get(day) ?? 0) + record.amount_cents);
  }
  const sorted = [...values.entries()].sort(([a], [b]) => Number(a) - Number(b));
  const max = Math.max(1, ...sorted.map(([, amount]) => amount));
  return sorted.map(([day, amount]) => ({ day, amount, height: Math.max(8, Math.round(amount / max * 100)) }));
});

function emptyLedger(value: string): MonthlyExpenseView {
  return {
    month: value,
    total_amount_cents: 0,
    personal_amount_cents: 0,
    collective_amount_cents: 0,
    category_totals: [],
    records: [],
  };
}

function monthText(date: Date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
}

function todayText() {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
}

function moveMonth(offset: number) {
	const year = Number(month.value.slice(0, 4));
	const monthNumber = Number(month.value.slice(5, 7));
  month.value = monthText(new Date(year, monthNumber - 1 + offset, 1));
}

function formatMoney(cents: number) {
  return new Intl.NumberFormat("zh-CN", {
    style: "currency",
    currency: "CNY",
    minimumFractionDigits: 2,
  }).format(cents / 100);
}

function categoryIcon(category: ExpenseCategory) {
  return categoryEmoji[category.icon_key] ?? "🧾";
}

function scopeText(scope: ExpenseScope) {
  return scope === "COLLECTIVE" ? "集体开销" : "个人开销";
}

function canManage(record: ExpenseRecord) {
  return record.member_id === session.member?.id ||
    (record.expense_scope === "COLLECTIVE" && canEditOther.value);
}

async function loadLedger() {
  loading.value = true;
  error.value = "";
  try {
    ledger.value = tab.value === "MINE"
      ? await getMyMonthlyExpenses(month.value)
      : await getCollectiveMonthlyExpenses(month.value);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "读取账本失败";
    ledger.value = emptyLedger(month.value);
  } finally {
    loading.value = false;
  }
}

function resetForm(scope: ExpenseScope) {
  editing.value = null;
  form.category_id = categories.value[0]?.id ?? 0;
  form.expense_scope = scope;
  form.title = "";
  form.amount = "";
  form.spent_on = todayText();
  form.note = "";
  delete form.version;
}

function openCreate() {
  resetForm(tab.value === "COLLECTIVE" ? "COLLECTIVE" : "PERSONAL");
  modalOpen.value = true;
}

function openEdit(record: ExpenseRecord) {
  editing.value = record;
  form.category_id = record.category_id;
  form.expense_scope = record.expense_scope;
  form.title = record.title;
  form.amount = (record.amount_cents / 100).toFixed(2);
  form.spent_on = record.spent_on.slice(0, 10);
  form.note = record.note;
  form.version = record.version;
  modalOpen.value = true;
}

async function save() {
  if (saving.value) return;
  saving.value = true;
  error.value = "";
  try {
    const payload = { ...form };
    if (editing.value) {
      await updateExpense(editing.value.id, payload);
    } else {
      await createExpense(payload);
    }
    modalOpen.value = false;
    await loadLedger();
    window.dispatchEvent(new Event("vhome:dashboard-changed"));
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "保存支出失败";
  } finally {
    saving.value = false;
  }
}

async function remove(record: ExpenseRecord) {
  if (!window.confirm(`确认删除“${record.title}”这条支出吗？`)) return;
  error.value = "";
  try {
    await deleteExpense(record.id, record.version);
    await loadLedger();
    window.dispatchEvent(new Event("vhome:dashboard-changed"));
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "删除支出失败";
  }
}

async function exportCurrentMonth(exportView?: ExpenseExportView) {
  error.value = "";
	const year = Number(month.value.slice(0, 4));
	const monthNumber = Number(month.value.slice(5, 7));
  const from = `${month.value}-01`;
  const lastDay = new Date(year, monthNumber, 0).getDate();
  const to = `${month.value}-${String(lastDay).padStart(2, "0")}`;
  const view: ExpenseExportView = exportView ?? (tab.value === "MINE" ? "MINE" : "COLLECTIVE");
  try {
    const blob = await downloadExpenseExport({ from, to, view });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    const labels: Record<ExpenseExportView, string> = {
      MINE: "个人开销",
      COLLECTIVE: "集体开销",
      HOUSEHOLD: "家庭总账",
    };
    link.download = `VHome-${month.value}-${labels[view]}.csv`;
    link.click();
    URL.revokeObjectURL(url);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "导出失败";
  }
}

watch([tab, month], () => void loadLedger());
onMounted(async () => {
  try {
    categories.value = await listExpenseCategories();
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "读取支出分类失败";
  }
  await loadLedger();
});
</script>

<template>
  <div class="expense-page page-stack">
    <section class="ledger-hero pixel-panel">
      <div>
        <p class="panel-kicker">FAMILY LEDGER</p>
        <h1>家庭记账本</h1>
        <p>钱花在哪里，家里的日子就清楚在哪里。</p>
      </div>
      <button class="primary-pixel-button compact" type="button" @click="openCreate">
        <Plus :size="17" /> 记一笔支出
      </button>
    </section>

    <p v-if="error" class="form-message error">{{ error }}</p>

    <section class="ledger-toolbar">
      <div class="ledger-tabs" role="tablist" aria-label="账本范围">
        <button :class="{ active: tab === 'MINE' }" type="button" @click="tab = 'MINE'">我的开销</button>
        <button :class="{ active: tab === 'COLLECTIVE' }" type="button" @click="tab = 'COLLECTIVE'">集体开销</button>
      </div>
      <div class="month-switcher">
        <button type="button" aria-label="上个月" @click="moveMonth(-1)"><ChevronLeft :size="18" /></button>
        <strong>{{ displayedMonth }}</strong>
        <button type="button" aria-label="下个月" :disabled="isCurrentMonth" @click="moveMonth(1)"><ChevronRight :size="18" /></button>
      </div>
      <button v-if="session.memberRole === 'OWNER'" class="secondary-button export-button" type="button" @click="exportCurrentMonth('HOUSEHOLD')">
        <Download :size="17" /> 导出家庭总账
      </button>
      <button class="secondary-button export-button" type="button" @click="exportCurrentMonth()">
        <Download :size="17" /> 导出本月
      </button>
    </section>

    <section class="ledger-metrics">
      <article><span class="metric-symbol green"><WalletCards :size="22" /></span><div><small>本月合计</small><strong>{{ formatMoney(ledger.total_amount_cents) }}</strong></div></article>
      <article><span class="metric-symbol yellow">👤</span><div><small>个人开销</small><strong>{{ formatMoney(ledger.personal_amount_cents) }}</strong></div></article>
      <article><span class="metric-symbol blue">🏡</span><div><small>集体开销</small><strong>{{ formatMoney(ledger.collective_amount_cents) }}</strong></div></article>
    </section>

    <section class="ledger-analysis">
      <article class="pixel-panel chart-card">
        <header><div><p class="panel-kicker">CATEGORY MIX</p><h2>分类占比</h2></div></header>
        <div v-if="ledger.category_totals.length" class="donut-layout">
          <div class="expense-donut" :style="{ background: categoryGradient }"><span><b>{{ ledger.records.length }}</b><small>笔支出</small></span></div>
          <div class="category-legend">
            <div v-for="(item, index) in ledger.category_totals" :key="item.category.id">
              <i :style="{ background: chartColors[index % chartColors.length] }" />
              <span>{{ categoryIcon(item.category) }} {{ item.category.name }}</span>
              <strong>{{ item.percentage }}%</strong>
            </div>
          </div>
        </div>
        <div v-else class="ledger-empty compact"><span>🌱</span><p>本月还没有分类数据</p></div>
      </article>

      <article class="pixel-panel chart-card">
        <header><div><p class="panel-kicker">DAILY SPENDING</p><h2>每日支出</h2></div></header>
        <div v-if="dailyTotals.length" class="daily-chart">
          <div v-for="item in dailyTotals" :key="item.day" class="daily-bar-item" :title="formatMoney(item.amount)">
            <div><i :style="{ height: `${item.height}%` }" /></div><small>{{ Number(item.day) }}日</small>
          </div>
        </div>
        <div v-else class="ledger-empty compact"><span>🪙</span><p>记下第一笔支出后，这里会出现趋势</p></div>
      </article>
    </section>

    <section class="pixel-panel record-panel">
      <header class="record-header">
        <div><p class="panel-kicker">EXPENSE RECORDS</p><h2>{{ tab === "MINE" ? "我的支出明细" : "家庭集体支出" }}</h2></div>
        <span>{{ ledger.records.length }} 笔</span>
      </header>
      <div v-if="loading" class="ledger-empty"><p>正在翻阅账本…</p></div>
      <div v-else-if="ledger.records.length" class="expense-record-list">
        <article v-for="record in ledger.records" :key="record.id" class="expense-record-row">
          <span class="record-icon">{{ categoryIcon(record.category) }}</span>
          <div class="record-main">
            <strong>{{ record.title }}</strong>
            <small>{{ record.spent_on.slice(0, 10) }} · {{ record.category.name }} · {{ record.member_name }}</small>
            <p v-if="record.note">{{ record.note }}</p>
          </div>
          <span class="scope-chip" :class="record.expense_scope.toLowerCase()">{{ scopeText(record.expense_scope) }}</span>
          <strong class="record-amount">{{ formatMoney(record.amount_cents) }}</strong>
          <div v-if="canManage(record)" class="record-actions">
            <button type="button" aria-label="编辑支出" @click="openEdit(record)"><Edit3 :size="16" /></button>
            <button class="danger" type="button" aria-label="删除支出" @click="remove(record)"><Trash2 :size="16" /></button>
          </div>
        </article>
      </div>
      <div v-else class="ledger-empty"><ReceiptText :size="32" /><p>这个月还没有支出记录</p><button type="button" @click="openCreate">记下第一笔</button></div>
    </section>

    <div v-if="modalOpen" class="expense-modal-backdrop" @click.self="modalOpen = false">
      <section class="expense-modal pixel-panel" role="dialog" aria-modal="true" :aria-label="editing ? '编辑支出' : '新增支出'">
        <header><div><p class="panel-kicker">{{ editing ? "EDIT EXPENSE" : "NEW EXPENSE" }}</p><h2>{{ editing ? "修改支出" : "记一笔支出" }}</h2></div><button type="button" aria-label="关闭" @click="modalOpen = false"><X :size="20" /></button></header>
        <p v-if="error" class="form-message error">{{ error }}</p>
        <form @submit.prevent="save">
          <div class="expense-form-grid">
            <label><span class="field-label">支出名称</span><input v-model.trim="form.title" maxlength="128" required placeholder="例如：本周蔬菜" /></label>
            <label><span class="field-label">金额（元）</span><input v-model.trim="form.amount" inputmode="decimal" required pattern="\d+(\.\d{1,2})?" placeholder="0.00" /></label>
            <label><span class="field-label">支出分类</span><select v-model.number="form.category_id" required><option v-for="category in categories" :key="category.id" :value="category.id">{{ categoryIcon(category) }} {{ category.name }}</option></select></label>
            <label><span class="field-label">发生日期</span><input v-model="form.spent_on" type="date" :max="todayText()" required /></label>
          </div>
          <fieldset class="scope-picker">
            <legend class="field-label">归属方式</legend>
            <label><input v-model="form.expense_scope" type="radio" value="PERSONAL" :disabled="!!editing && editing.member_id !== session.member?.id" /><span>👤 个人开销<small>只在自己的账本中展示</small></span></label>
            <label><input v-model="form.expense_scope" type="radio" value="COLLECTIVE" /><span>🏡 集体开销<small>家庭成员都能查看</small></span></label>
          </fieldset>
          <label><span class="field-label">备注（可选）</span><textarea v-model.trim="form.note" maxlength="500" rows="3" placeholder="记录账单用途或其他信息" /></label>
          <footer><button class="secondary-button" type="button" @click="modalOpen = false">取消</button><button class="primary-pixel-button compact" type="submit" :disabled="saving">{{ saving ? "保存中…" : "保存支出" }}</button></footer>
        </form>
      </section>
    </div>
  </div>
</template>

<style scoped>
.ledger-hero{display:flex;align-items:center;justify-content:space-between;padding:26px 30px;background:linear-gradient(110deg,#fff9e9,#edf3d5)}
.ledger-hero h1{margin:5px 0 8px;font-size:30px}.ledger-hero>div>p:last-child{color:#806957;font-size:14px}.ledger-toolbar{display:flex;align-items:center;gap:14px;flex-wrap:wrap}.ledger-tabs{display:flex;padding:4px;border:2px solid #c9a974;border-radius:8px;background:#f4e5bd}.ledger-tabs button{padding:9px 18px;border:0;border-radius:5px;cursor:pointer;font-weight:800;background:transparent}.ledger-tabs button.active{color:#fff;background:var(--grass-500);box-shadow:0 2px 0 var(--wood-600)}.month-switcher{display:flex;align-items:center;gap:12px;margin-left:auto;padding:5px;border:2px solid #d3b77e;border-radius:7px;background:#fffaf0}.month-switcher button,.record-actions button,.expense-modal>header button{display:grid;border:0;cursor:pointer;place-items:center;background:transparent}.month-switcher button{width:32px;height:32px;border-radius:4px}.month-switcher button:hover{background:var(--cream-200)}.export-button{min-height:44px}.ledger-metrics{display:grid;grid-template-columns:repeat(3,1fr);gap:15px}.ledger-metrics article{display:flex;align-items:center;gap:14px;padding:19px 21px;border:2px solid #d2b57e;border-radius:8px;background:#fffaf0;box-shadow:0 3px 0 rgba(83,50,37,.13)}.ledger-metrics article div{display:grid;gap:3px}.ledger-metrics small{color:#8c7562}.ledger-metrics strong{font-size:23px}.metric-symbol{display:grid;width:46px;height:46px;border-radius:9px;place-items:center;font-size:21px}.metric-symbol.green{color:#fff;background:#7ea653}.metric-symbol.yellow{background:#f4d481}.metric-symbol.blue{background:#aed5d9}.ledger-analysis{display:grid;grid-template-columns:1fr 1fr;gap:18px}.chart-card{min-height:280px;padding:22px}.chart-card h2,.record-header h2,.expense-modal h2{margin-top:4px;font-size:20px}.donut-layout{display:flex;align-items:center;gap:28px;height:200px}.expense-donut{display:grid;width:145px;height:145px;flex:0 0 145px;border:3px solid var(--wood-500);border-radius:50%;place-items:center;box-shadow:inset 0 0 0 4px #fff}.expense-donut:before{position:absolute;width:83px;height:83px;border-radius:50%;content:"";background:#fffaf0}.expense-donut span{z-index:1;display:grid;text-align:center}.expense-donut b{font-size:24px}.expense-donut small{color:#917764}.category-legend{display:grid;width:100%;gap:9px}.category-legend div{display:grid;grid-template-columns:10px 1fr auto;align-items:center;gap:8px;font-size:13px}.category-legend i{width:9px;height:9px;border-radius:2px}.daily-chart{display:flex;height:190px;align-items:end;gap:8px;padding:20px 4px 0;overflow-x:auto;border-bottom:2px solid #d9c69b}.daily-bar-item{display:grid;min-width:30px;height:100%;grid-template-rows:1fr auto;gap:6px;text-align:center}.daily-bar-item>div{display:flex;height:100%;align-items:end;justify-content:center}.daily-bar-item i{display:block;width:18px;min-height:8px;border:2px solid #567b42;border-bottom:0;border-radius:4px 4px 0 0;background:#8eb25b}.daily-bar-item small{color:#89715e;font-size:10px}.record-panel{overflow:hidden}.record-header{display:flex;align-items:center;justify-content:space-between;padding:21px 24px;border-bottom:2px solid #e4d0a0}.record-header>span{padding:5px 10px;border-radius:12px;color:#6d593f;background:#f2e2b9;font-size:12px;font-weight:800}.expense-record-list{display:grid}.expense-record-row{display:grid;grid-template-columns:46px minmax(160px,1fr) auto 130px 70px;align-items:center;gap:14px;padding:15px 22px;border-bottom:1px solid #ead8ae}.expense-record-row:last-child{border-bottom:0}.record-icon{display:grid;width:42px;height:42px;border-radius:8px;place-items:center;background:#f4e3b9;font-size:21px}.record-main{display:grid;gap:3px;min-width:0}.record-main small,.record-main p{overflow:hidden;color:#8c7562;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.record-main p{color:#a17f62}.scope-chip{padding:5px 9px;border-radius:12px;font-size:11px;font-weight:800}.scope-chip.personal{color:#6d5b3a;background:#f3dfac}.scope-chip.collective{color:#3c6e71;background:#d8eeee}.record-amount{text-align:right;font-size:16px}.record-actions{display:flex;justify-content:end;gap:5px}.record-actions button{width:30px;height:30px;border:1px solid #d4ba85;border-radius:5px;background:#fff9e9}.record-actions button.danger{color:var(--red)}.ledger-empty{display:grid;min-height:180px;align-content:center;justify-items:center;gap:8px;color:#927b67}.ledger-empty.compact{min-height:190px}.ledger-empty button{border:0;cursor:pointer;color:var(--grass-600);background:transparent;font-weight:800}.expense-modal-backdrop{position:fixed;z-index:80;inset:0;display:grid;padding:20px;overflow-y:auto;place-items:center;background:rgba(47,34,25,.54)}.expense-modal{width:min(680px,100%);padding:25px;background:#fffaf0}.expense-modal>header{display:flex;align-items:start;justify-content:space-between;margin-bottom:22px}.expense-modal>header button{width:36px;height:36px;border:1px solid #d3b77e;border-radius:5px}.expense-form-grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}.expense-modal input,.expense-modal select,.expense-modal textarea{width:100%;border:2px solid #d2b57e;border-radius:7px;outline:0;color:var(--wood-800);background:#fffdf6}.expense-modal input,.expense-modal select{height:46px;padding:0 12px}.expense-modal textarea{padding:11px 12px;resize:vertical}.expense-modal input:focus,.expense-modal select:focus,.expense-modal textarea:focus{border-color:var(--grass-500);box-shadow:0 0 0 3px rgba(99,135,68,.13)}.scope-picker{display:grid;grid-template-columns:1fr 1fr;gap:10px;margin:18px 0;padding:0;border:0}.scope-picker legend{grid-column:1/-1}.scope-picker label{display:flex;align-items:center;gap:10px;padding:12px;border:2px solid #dbc18c;border-radius:7px;cursor:pointer;background:#fff8e7}.scope-picker label>span{display:grid;font-weight:800}.scope-picker small{color:#947b66;font-size:11px;font-weight:400}.expense-modal footer{display:flex;justify-content:end;gap:10px;margin-top:22px}.expense-modal footer .primary-pixel-button{min-width:120px}
.expense-donut{position:relative}
@media(max-width:900px){.ledger-analysis{grid-template-columns:1fr}.expense-record-row{grid-template-columns:44px 1fr auto}.scope-chip{grid-column:2}.record-amount{grid-column:3;grid-row:1;text-align:right}.record-actions{grid-column:3;grid-row:2}.ledger-metrics{grid-template-columns:1fr}}
@media(max-width:620px){.ledger-hero{align-items:start;flex-direction:column;gap:18px;padding:22px}.ledger-toolbar{align-items:stretch}.month-switcher{order:1;width:100%;justify-content:space-between;margin:0}.ledger-tabs{order:2;flex:1}.ledger-tabs button{flex:1;padding-inline:9px}.export-button{order:2}.donut-layout{height:auto;align-items:start;flex-direction:column;padding-top:20px}.chart-card{min-height:330px}.expense-record-row{padding:14px;gap:9px}.expense-form-grid,.scope-picker{grid-template-columns:1fr}.scope-picker legend{grid-column:auto}.expense-modal{padding:20px}.record-main small{white-space:normal}}
</style>
