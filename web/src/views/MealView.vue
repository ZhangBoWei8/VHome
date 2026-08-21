<script setup lang="ts">
import {
  AlertTriangle,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  CirclePlus,
  Pencil,
  Plus,
  RotateCcw,
  Search,
  Trash2,
  Utensils,
  Wheat,
  X,
} from "@lucide/vue";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";

import {
  APIError,
  createFood,
  createMyMealRecord,
  deleteFood,
  deleteMyMealRecord,
  getMemberMealToday,
  getMyMealCalendar,
  getMyMealDay,
  listFoods,
  listMealMemberOptions,
  restoreFood,
  updateFood,
  updateMyMealRecord,
  type Food,
  type FoodIconType,
  type MealCalendar,
  type MealDay,
  type MealMemberOption,
  type MealRecord,
  type MealType,
} from "@/api";
import pantryIcons from "@/assets/pantry-icons.png";
import { useSessionStore } from "@/stores/session";

const session = useSessionStore();

const iconOrder = [
  "milk", "egg", "leafy-vegetable", "potato", "scallion",
  "pork", "beef", "chicken", "fish", "fruit",
  "chips", "canned-food", "rice", "bread", "seasoning",
  "generic-food", "drink",
];
const iconLabels: Record<string, string> = {
  milk: "牛奶", egg: "鸡蛋", "leafy-vegetable": "叶菜", potato: "土豆",
  scallion: "葱", pork: "猪肉", beef: "牛肉", chicken: "鸡肉", fish: "鱼类",
  fruit: "水果", chips: "零食", "canned-food": "罐头", rice: "米粮",
  bread: "面包", seasoning: "调味品", "generic-food": "通用食物", drink: "饮品",
};
const avatarEmoji: Record<string, string> = {
  man: "👨", woman: "👩", boy: "👦", girl: "👧", dog: "🐶", initials: "🌻",
};
const mealSections: Array<{ type: MealType; label: string; icon: string; hint: string }> = [
  { type: "BREAKFAST", label: "早餐", icon: "☀️", hint: "开启一天的能量" },
  { type: "LUNCH", label: "午餐", icon: "🍚", hint: "午间补充" },
  { type: "DINNER", label: "晚餐", icon: "🌙", hint: "温暖的一餐" },
  { type: "SNACK", label: "加餐", icon: "🍎", hint: "零食与饮品" },
];

function iconStyle(key: string) {
  const index = Math.max(0, iconOrder.indexOf(key));
  return {
    backgroundImage: `url(${pantryIcons})`,
    backgroundSize: "500% 400%",
    backgroundPosition: `${(index % 5) * 25}% ${Math.floor(index / 5) * (100 / 3)}%`,
  };
}

function localDateString(date = new Date()) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function shiftISODate(value: string, amount: number) {
  const date = new Date(`${value}T00:00:00Z`);
  date.setUTCDate(date.getUTCDate() + amount);
  return date.toISOString().slice(0, 10);
}

function formatDay(value: string) {
  const date = new Date(`${value}T00:00:00`);
  if (value === today) return "今天";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "long", day: "numeric", weekday: "short",
  }).format(date);
}

function formatNumber(value: number) {
  return Number(value.toFixed(2)).toString();
}

function errorMessage(cause: unknown, fallback: string) {
  return cause instanceof Error ? cause.message : fallback;
}

const today = localDateString();
const memberOptions = ref<MealMemberOption[]>([]);
const selectedMemberID = ref(0);
const mySelectedDate = ref(today);
const day = ref<MealDay | null>(null);
const calendar = ref<MealCalendar | null>(null);
const calendarMonth = ref(today.slice(0, 7));
const foods = ref<Food[]>([]);
const loading = ref(true);
const error = ref("");

const currentMember = computed(() =>
  memberOptions.value.find((member) => member.id === selectedMemberID.value),
);
const viewingSelf = computed(() => currentMember.value?.is_current === true);
const selectedDate = computed(() => viewingSelf.value ? mySelectedDate.value : today);
const recordsByType = computed(() => {
  const result: Record<MealType, MealRecord[]> = {
    BREAKFAST: [], LUNCH: [], DINNER: [], SNACK: [],
  };
  for (const record of day.value?.records ?? []) result[record.meal_type].push(record);
  return result;
});

async function loadFoods() {
  foods.value = await listFoods("ACTIVE");
}

async function loadCalendar() {
  if (!viewingSelf.value) {
    calendar.value = null;
    return;
  }
  calendar.value = await getMyMealCalendar(calendarMonth.value);
}

async function loadDay() {
  if (!selectedMemberID.value) return;
  loading.value = true;
  error.value = "";
  try {
    day.value = viewingSelf.value
      ? await getMyMealDay(mySelectedDate.value)
      : await getMemberMealToday(selectedMemberID.value);
  } catch (cause) {
    error.value = errorMessage(cause, "读取饮食记录失败");
    day.value = null;
  } finally {
    loading.value = false;
  }
}

async function initialize() {
  loading.value = true;
  error.value = "";
  try {
    const [members] = await Promise.all([listMealMemberOptions(), loadFoods()]);
    memberOptions.value = members;
    selectedMemberID.value = members.find((member) => member.is_current)?.id ?? members[0]?.id ?? 0;
    await Promise.all([loadDay(), loadCalendar()]);
  } catch (cause) {
    error.value = errorMessage(cause, "饮食日历加载失败");
    loading.value = false;
  }
}

async function selectMember(member: MealMemberOption) {
  if (member.id === selectedMemberID.value) return;
  selectedMemberID.value = member.id;
  await Promise.all([loadDay(), loadCalendar()]);
}

async function changeDate(amount: number) {
  const next = shiftISODate(mySelectedDate.value, amount);
  if (next > today) return;
  mySelectedDate.value = next;
  const month = next.slice(0, 7);
  if (month !== calendarMonth.value) calendarMonth.value = month;
  await Promise.all([loadDay(), loadCalendar()]);
}

async function chooseDate(value: string) {
  if (value > today) return;
  mySelectedDate.value = value;
  await loadDay();
}

function monthParts(value: string): [number, number] {
  const [year = "0", month = "0"] = value.split("-");
  return [Number(year), Number(month)];
}

function shiftMonth(amount: number) {
  const [year, month] = monthParts(calendarMonth.value);
  const date = new Date(Date.UTC(year, month - 1 + amount, 1));
  const next = date.toISOString().slice(0, 7);
  if (next > today.slice(0, 7)) return;
  calendarMonth.value = next;
  void loadCalendar().catch((cause) => { error.value = errorMessage(cause, "读取月历失败"); });
}

const calendarTitle = computed(() => {
  const [year, month] = monthParts(calendarMonth.value);
  return `${year} 年 ${month} 月`;
});
const calendarCells = computed(() => {
  const [year, month] = monthParts(calendarMonth.value);
  const first = new Date(Date.UTC(year, month - 1, 1));
  const leading = (first.getUTCDay() + 6) % 7;
  const total = new Date(Date.UTC(year, month, 0)).getUTCDate();
  const summary = new Map((calendar.value?.days ?? []).map((item) => [item.date, item]));
  const cells: Array<null | { date: string; number: number; future: boolean; calories: number | null; incomplete: boolean }> = Array(leading).fill(null);
  for (let number = 1; number <= total; number += 1) {
    const date = `${year}-${String(month).padStart(2, "0")}-${String(number).padStart(2, "0")}`;
    const info = summary.get(date);
    cells.push({
      date, number, future: date > today,
      calories: info ? info.calories : null,
      incomplete: info?.nutrition_incomplete ?? false,
    });
  }
  return cells;
});

const recordModal = ref(false);
const editingRecord = ref<MealRecord | null>(null);
const savingRecord = ref(false);
const recordError = ref("");
const recordForm = ref({ meal_type: "BREAKFAST" as MealType, food_id: 0, weight_grams: 100 });
const selectedFood = computed(() => foods.value.find((food) => food.id === recordForm.value.food_id));
const intakePreview = computed(() => {
  const food = selectedFood.value;
  const factor = Number(recordForm.value.weight_grams) / 100;
  if (!food || !Number.isFinite(factor) || factor <= 0) return null;
  return {
    calories: food.calories_per_100g * factor,
    carbohydrate: food.carbohydrate_per_100g == null ? null : food.carbohydrate_per_100g * factor,
    protein: food.protein_per_100g == null ? null : food.protein_per_100g * factor,
    fat: food.fat_per_100g == null ? null : food.fat_per_100g * factor,
  };
});

function openRecordForm(mealType: MealType, record?: MealRecord) {
  editingRecord.value = record ?? null;
  recordError.value = "";
  recordForm.value = {
    meal_type: record?.meal_type ?? mealType,
    food_id: record?.food_id ?? foods.value[0]?.id ?? 0,
    weight_grams: record?.weight_grams ?? 100,
  };
  recordModal.value = true;
}

async function saveRecord() {
  if (!selectedFood.value) {
    recordError.value = "请选择食品";
    return;
  }
  savingRecord.value = true;
  recordError.value = "";
  try {
    const input = {
      meal_date: mySelectedDate.value,
      meal_type: recordForm.value.meal_type,
      food_id: recordForm.value.food_id,
      weight_grams: Number(recordForm.value.weight_grams),
      ...(editingRecord.value ? { version: editingRecord.value.version } : {}),
    };
    if (editingRecord.value) await updateMyMealRecord(editingRecord.value.id, input);
    else await createMyMealRecord(input);
    recordModal.value = false;
    await Promise.all([loadDay(), loadCalendar()]);
  } catch (cause) {
    recordError.value = errorMessage(cause, "保存饮食记录失败");
  } finally {
    savingRecord.value = false;
  }
}

async function removeRecord(record: MealRecord) {
  if (!window.confirm(`确定删除“${record.food_name_snapshot}”这条记录吗？`)) return;
  try {
    await deleteMyMealRecord(record.id, record.version);
    await Promise.all([loadDay(), loadCalendar()]);
  } catch (cause) {
    error.value = errorMessage(cause, "删除饮食记录失败");
  }
}

const foodManager = ref(false);
const foodScope = ref<"ACTIVE" | "DELETED">("ACTIVE");
const managedFoods = ref<Food[]>([]);
const foodSearch = ref("");
const foodManagerError = ref("");
const filteredManagedFoods = computed(() => {
  const keyword = foodSearch.value.trim().toLowerCase();
  return keyword ? managedFoods.value.filter((food) => food.name.toLowerCase().includes(keyword)) : managedFoods.value;
});

function canManage(food: Food) {
  return session.memberRole === "OWNER" || session.memberRole === "ADMIN" ||
    (food.source === "USER" && food.created_by === session.member?.id);
}

async function loadManagedFoods() {
  foodManagerError.value = "";
  try {
    managedFoods.value = await listFoods(foodScope.value);
  } catch (cause) {
    foodManagerError.value = errorMessage(cause, "读取食品失败");
  }
}

async function openFoodManager() {
  foodScope.value = "ACTIVE";
  foodSearch.value = "";
  foodManager.value = true;
  await loadManagedFoods();
}

watch(foodScope, () => void loadManagedFoods());

const foodFormModal = ref(false);
const editingFood = ref<Food | null>(null);
const savingFood = ref(false);
const foodFormError = ref("");
const foodImage = ref<File | null>(null);
const foodImagePreview = ref("");
let foodImageObjectURL = "";
const foodForm = ref({
  name: "", calories: "", carbohydrate: "", protein: "", fat: "",
  icon_type: "BUILTIN" as FoodIconType, icon_value: "generic-food",
});

watch(foodImage, (file) => {
  if (foodImageObjectURL) globalThis.URL.revokeObjectURL(foodImageObjectURL);
  foodImageObjectURL = file ? globalThis.URL.createObjectURL(file) : "";
  foodImagePreview.value = foodImageObjectURL;
});

onBeforeUnmount(() => {
  if (foodImageObjectURL) globalThis.URL.revokeObjectURL(foodImageObjectURL);
});

function openFoodForm(food?: Food) {
  editingFood.value = food ?? null;
  foodImage.value = null;
  foodFormError.value = "";
  foodForm.value = {
    name: food?.name ?? "",
    calories: food?.calories_per_100g.toString() ?? "",
    carbohydrate: food?.carbohydrate_per_100g?.toString() ?? "",
    protein: food?.protein_per_100g?.toString() ?? "",
    fat: food?.fat_per_100g?.toString() ?? "",
    icon_type: food?.icon_type ?? "BUILTIN",
    icon_value: food?.icon_value ?? "generic-food",
  };
  foodFormModal.value = true;
}

function buildFoodFormData(confirmMismatch: boolean) {
  const form = new FormData();
  form.append("name", foodForm.value.name);
  form.append("calories_per_100g", foodForm.value.calories);
  form.append("carbohydrate_per_100g", foodForm.value.carbohydrate);
  form.append("protein_per_100g", foodForm.value.protein);
  form.append("fat_per_100g", foodForm.value.fat);
  form.append("icon_type", foodForm.value.icon_type);
  if (foodForm.value.icon_type === "BUILTIN") form.append("icon_value", foodForm.value.icon_value);
  if (foodImage.value) form.append("image", foodImage.value);
  if (editingFood.value) form.append("version", String(editingFood.value.version));
  form.append("confirm_nutrition_mismatch", String(confirmMismatch));
  return form;
}

async function persistFood(confirmMismatch = false): Promise<void> {
  try {
    const form = buildFoodFormData(confirmMismatch);
    if (editingFood.value) await updateFood(editingFood.value.id, form);
    else await createFood(form);
  } catch (cause) {
    if (cause instanceof APIError && cause.code === "NUTRITION_CONFIRMATION_REQUIRED" && !confirmMismatch) {
      if (window.confirm("填写的热量与三大营养素估算值差异较大，仍要保存吗？")) {
        await persistFood(true);
        return;
      }
    }
    throw cause;
  }
}

async function saveFood() {
  if (
    foodForm.value.icon_type === "UPLOAD" &&
    !foodImage.value &&
    editingFood.value?.icon_type !== "UPLOAD"
  ) {
    foodFormError.value = "请选择一张食品图片";
    return;
  }
  savingFood.value = true;
  foodFormError.value = "";
  try {
    await persistFood();
    foodFormModal.value = false;
    await Promise.all([loadManagedFoods(), loadFoods()]);
  } catch (cause) {
    foodFormError.value = errorMessage(cause, "保存食品失败");
  } finally {
    savingFood.value = false;
  }
}

async function removeFood(food: Food) {
  if (!window.confirm(`确定将“${food.name}”移入回收站吗？历史饮食记录不会受到影响。`)) return;
  try {
    await deleteFood(food.id, food.version);
    await Promise.all([loadManagedFoods(), loadFoods()]);
  } catch (cause) {
    foodManagerError.value = errorMessage(cause, "删除食品失败");
  }
}

async function recoverFood(food: Food) {
  try {
    await restoreFood(food.id, food.version);
    await Promise.all([loadManagedFoods(), loadFoods()]);
  } catch (cause) {
    foodManagerError.value = errorMessage(cause, "恢复食品失败");
  }
}

onMounted(initialize);
</script>

<template>
  <div class="meal-page page-stack">
    <section class="meal-hero">
      <div>
        <p class="eyebrow"><Wheat :size="15" /> FAMILY TABLE</p>
        <h1>饮食日历</h1>
        <p>从每一餐开始，轻松了解家人的每日营养。</p>
      </div>
      <div class="meal-hero-actions">
        <button class="secondary-button" type="button" @click="openFoodManager">
          <Utensils :size="17" /> 食品管理
        </button>
        <button v-if="viewingSelf" class="primary-pixel-button compact" type="button" @click="openRecordForm('BREAKFAST')">
          <Plus :size="17" /> 记录一餐
        </button>
      </div>
    </section>

    <p v-if="error" class="form-message error">{{ error }}</p>

    <section class="pixel-panel meal-member-switcher">
      <span class="member-switcher-label">查看成员</span>
      <div class="meal-member-tabs">
        <button
          v-for="member in memberOptions"
          :key="member.id"
          type="button"
          :class="{ active: member.id === selectedMemberID }"
          @click="selectMember(member)"
        >
          <span>{{ avatarEmoji[member.avatar_key] ?? member.display_name.slice(0, 1) }}</span>
          <strong>{{ member.is_current ? "我的饮食" : member.display_name }}</strong>
          <small>{{ member.is_current ? "可查看历史并记录" : "仅查看今天" }}</small>
        </button>
      </div>
    </section>

    <div class="meal-layout">
      <main class="meal-main-column">
        <section class="meal-date-bar pixel-panel">
          <div>
            <p class="panel-kicker"><CalendarDays :size="14" /> DAILY LOG</p>
            <h2>{{ formatDay(selectedDate) }}</h2>
            <small>{{ selectedDate }} · {{ day?.member.display_name ?? currentMember?.display_name }}</small>
          </div>
          <div v-if="viewingSelf" class="date-nav-buttons">
            <button type="button" aria-label="查看前一天饮食" @click="changeDate(-1)"><ChevronLeft /></button>
            <button type="button" :disabled="mySelectedDate >= today" aria-label="查看后一天饮食" @click="changeDate(1)"><ChevronRight /></button>
          </div>
          <span v-else class="today-only-chip">今日只读</span>
        </section>

        <section v-if="day" class="meal-summary-grid">
          <article class="meal-summary-card calories">
            <span>🔥</span><div><small>今日热量</small><strong>{{ formatNumber(day.summary.calories) }}</strong><em>kcal</em></div>
          </article>
          <article class="meal-summary-card carbs">
            <span>🌾</span><div><small>碳水</small><strong>{{ formatNumber(day.summary.carbohydrate) }}</strong><em>g</em></div>
          </article>
          <article class="meal-summary-card protein">
            <span>🥚</span><div><small>蛋白质</small><strong>{{ formatNumber(day.summary.protein) }}</strong><em>g</em></div>
          </article>
          <article class="meal-summary-card fat">
            <span>🥑</span><div><small>脂肪</small><strong>{{ formatNumber(day.summary.fat) }}</strong><em>g</em></div>
          </article>
        </section>

        <div v-if="day?.summary.nutrition_incomplete" class="nutrition-warning">
          <AlertTriangle :size="18" />
          <div>
            <strong>当天存在 {{ day.summary.incomplete_record_count }} 条营养信息不完整的记录</strong>
            <small>热量仍会统计，但三大营养素合计可能不准确。</small>
          </div>
        </div>

        <section v-if="loading" class="pixel-panel meal-loading">正在整理今天的餐桌…</section>
        <section v-else-if="day" class="meal-sections">
          <article v-for="section in mealSections" :key="section.type" class="pixel-panel meal-section">
            <header>
              <div class="meal-section-title">
                <span>{{ section.icon }}</span>
                <div><h3>{{ section.label }}</h3><small>{{ section.hint }}</small></div>
              </div>
              <button v-if="viewingSelf" type="button" class="meal-add-button" @click="openRecordForm(section.type)">
                <CirclePlus :size="17" /> 添加
              </button>
            </header>

            <div v-if="recordsByType[section.type].length" class="meal-record-list">
              <div v-for="record in recordsByType[section.type]" :key="record.id" class="meal-record-row">
                <img v-if="record.icon_type_snapshot === 'UPLOAD'" :src="`/uploads/${record.icon_value_snapshot}`" :alt="record.food_name_snapshot" />
                <span v-else class="meal-food-icon" :style="iconStyle(record.icon_value_snapshot)" />
                <div class="meal-record-copy">
                  <strong>{{ record.food_name_snapshot }}</strong>
                  <small>{{ formatNumber(record.weight_grams) }} g · 每100g {{ formatNumber(record.calories_per_100g_snapshot) }} kcal</small>
                </div>
                <div class="meal-record-nutrition">
                  <strong>{{ formatNumber(record.calories) }} kcal</strong>
                  <small v-if="!record.nutrition_incomplete">
                    碳水 {{ formatNumber(record.carbohydrate ?? 0) }}g · 蛋白 {{ formatNumber(record.protein ?? 0) }}g · 脂肪 {{ formatNumber(record.fat ?? 0) }}g
                  </small>
                  <small v-else class="incomplete-text">部分营养信息未录入</small>
                </div>
                <div v-if="viewingSelf" class="meal-record-actions">
                  <button type="button" aria-label="编辑这条饮食记录" data-tooltip-placement="top" @click="openRecordForm(record.meal_type, record)"><Pencil :size="15" /></button>
                  <button type="button" aria-label="删除这条饮食记录" data-tooltip-placement="top-left" @click="removeRecord(record)"><Trash2 :size="15" /></button>
                </div>
              </div>
            </div>
            <button v-else-if="viewingSelf" type="button" class="meal-empty-row" @click="openRecordForm(section.type)">
              <span>＋</span> 还没有记录，点击添加{{ section.label }}
            </button>
            <p v-else class="meal-empty-readonly">今天还没有{{ section.label }}记录</p>
          </article>
        </section>
      </main>

      <aside v-if="viewingSelf" class="pixel-panel meal-calendar-panel">
        <header>
          <button type="button" aria-label="查看上个月饮食" @click="shiftMonth(-1)"><ChevronLeft :size="17" /></button>
          <div><p class="panel-kicker">HISTORY</p><strong>{{ calendarTitle }}</strong></div>
          <button type="button" :disabled="calendarMonth >= today.slice(0, 7)" aria-label="查看下个月饮食" data-tooltip-placement="left" @click="shiftMonth(1)"><ChevronRight :size="17" /></button>
        </header>
        <div class="calendar-weekdays"><span v-for="label in ['一','二','三','四','五','六','日']" :key="label">{{ label }}</span></div>
        <div class="calendar-grid">
          <span v-for="(cell, index) in calendarCells" :key="cell?.date ?? `blank-${index}`" class="calendar-cell-wrap">
            <button
              v-if="cell"
              type="button"
              class="calendar-day"
              :class="{ selected: cell.date === mySelectedDate, today: cell.date === today, future: cell.future, recorded: cell.calories !== null }"
              :disabled="cell.future"
              @click="chooseDate(cell.date)"
            >
              <strong>{{ cell.number }}</strong>
              <small v-if="cell.calories !== null">{{ Math.round(cell.calories) }}</small>
              <i v-if="cell.incomplete" title="营养信息不完整">!</i>
            </button>
          </span>
        </div>
        <div class="calendar-legend"><span><i class="recorded-dot" />有记录</span><span><i class="warning-dot" />营养待补全</span></div>
      </aside>
    </div>

    <div v-if="recordModal" class="modal-backdrop" @click.self="recordModal = false">
      <section class="modal-card meal-record-modal">
        <header>
          <div><p class="panel-kicker">MEAL ENTRY</p><h2>{{ editingRecord ? "编辑饮食记录" : "记录一餐" }}</h2></div>
          <button type="button" aria-label="关闭饮食记录窗口" data-tooltip-placement="left" @click="recordModal = false"><X /></button>
        </header>
        <form class="material-form" @submit.prevent="saveRecord">
          <p v-if="recordError" class="form-message error">{{ recordError }}</p>
          <div class="form-grid">
            <label><span>用餐日期</span><input :value="mySelectedDate" type="date" disabled /></label>
            <label><span>餐次</span><select v-model="recordForm.meal_type"><option v-for="section in mealSections" :key="section.type" :value="section.type">{{ section.label }}</option></select></label>
          </div>
          <label>
            <span>选择食品</span>
            <select v-model="recordForm.food_id" required>
              <option :value="0" disabled>请选择食品</option>
              <option v-for="food in foods" :key="food.id" :value="food.id">{{ food.name }} · {{ formatNumber(food.calories_per_100g) }} kcal/100g</option>
            </select>
            <small class="form-help">食品营养信息只做展示；需要调整时请前往食品管理。</small>
          </label>
          <label><span>食用重量（g）</span><input v-model.number="recordForm.weight_grams" type="number" min="0.01" max="100000" step="0.01" required /></label>
          <div v-if="intakePreview" class="meal-preview">
            <div><small>预计热量</small><strong>{{ formatNumber(intakePreview.calories) }} kcal</strong></div>
            <div><small>碳水</small><strong>{{ intakePreview.carbohydrate == null ? "未录入" : `${formatNumber(intakePreview.carbohydrate)} g` }}</strong></div>
            <div><small>蛋白质</small><strong>{{ intakePreview.protein == null ? "未录入" : `${formatNumber(intakePreview.protein)} g` }}</strong></div>
            <div><small>脂肪</small><strong>{{ intakePreview.fat == null ? "未录入" : `${formatNumber(intakePreview.fat)} g` }}</strong></div>
          </div>
          <div class="modal-actions"><button type="button" class="secondary-button" @click="recordModal = false">取消</button><button class="primary-pixel-button compact" :disabled="savingRecord">{{ savingRecord ? "保存中…" : "保存记录" }}</button></div>
        </form>
      </section>
    </div>

    <div v-if="foodManager" class="modal-backdrop" @click.self="foodManager = false">
      <section class="modal-card food-manager-modal">
        <header>
          <div><p class="panel-kicker">FOOD LIBRARY</p><h2>食品管理</h2><small>食品是饮食记录使用的营养数据，不会直接改变物料库存。</small></div>
          <button type="button" aria-label="关闭食品管理" data-tooltip-placement="left" @click="foodManager = false"><X /></button>
        </header>
        <div class="food-manager-body">
          <p v-if="foodManagerError" class="form-message error">{{ foodManagerError }}</p>
          <div class="food-manager-toolbar">
            <div class="inventory-tabs"><button :class="{ active: foodScope === 'ACTIVE' }" @click="foodScope = 'ACTIVE'">可用食品</button><button :class="{ active: foodScope === 'DELETED' }" @click="foodScope = 'DELETED'">回收站</button></div>
            <button v-if="foodScope === 'ACTIVE'" class="primary-pixel-button compact" type="button" @click="openFoodForm()"><Plus :size="16" /> 自定义食品</button>
          </div>
          <div class="search-box food-search"><Search :size="17" /><input v-model="foodSearch" placeholder="搜索食品…" /></div>
          <div class="food-library-list">
            <article v-for="food in filteredManagedFoods" :key="food.id" class="food-library-row">
              <img v-if="food.icon_type === 'UPLOAD'" :src="`/uploads/${food.icon_value}`" :alt="food.name" />
              <span v-else class="meal-food-icon large" :style="iconStyle(food.icon_value)" />
              <div class="food-library-copy"><strong>{{ food.name }}</strong><small>{{ food.source === 'BUILTIN' ? '内置食品' : '自定义食品' }} · {{ formatNumber(food.calories_per_100g) }} kcal/100g</small><small>碳水 {{ food.carbohydrate_per_100g ?? '未录入' }} · 蛋白 {{ food.protein_per_100g ?? '未录入' }} · 脂肪 {{ food.fat_per_100g ?? '未录入' }}</small></div>
              <span v-if="!food.nutrition_complete" class="food-status-chip warning">营养待补全</span>
              <span v-else-if="food.nutrition_mismatch" class="food-status-chip danger">热量需留意</span>
              <span v-else class="food-status-chip safe">营养完整</span>
              <div class="food-library-actions">
                <template v-if="foodScope === 'ACTIVE' && canManage(food)"><button type="button" @click="openFoodForm(food)"><Pencil :size="15" /> 编辑</button><button type="button" @click="removeFood(food)"><Trash2 :size="15" /> 删除</button></template>
                <button v-else-if="foodScope === 'DELETED'" type="button" @click="recoverFood(food)"><RotateCcw :size="15" /> 恢复</button>
              </div>
            </article>
            <div v-if="!filteredManagedFoods.length" class="food-library-empty">{{ foodSearch ? "没有匹配的食品" : foodScope === 'DELETED' ? "回收站是空的" : "还没有可用食品" }}</div>
          </div>
        </div>
      </section>
    </div>

    <div v-if="foodFormModal" class="modal-backdrop food-form-layer" @click.self="foodFormModal = false">
      <section class="modal-card material-modal">
        <header><div><p class="panel-kicker">FOOD PROFILE</p><h2>{{ editingFood ? "编辑食品" : "自定义食品" }}</h2></div><button type="button" aria-label="关闭食品编辑窗口" data-tooltip-placement="left" @click="foodFormModal = false"><X /></button></header>
        <form class="material-form" @submit.prevent="saveFood">
          <p v-if="foodFormError" class="form-message error">{{ foodFormError }}</p>
          <div class="form-grid"><label><span>食品名称</span><input v-model="foodForm.name" maxlength="128" required /></label><label><span>每100克热量（kcal）</span><input v-model="foodForm.calories" type="number" min="0" max="1000" step="0.01" required /></label></div>
          <div class="form-grid nutrition-grid"><label><span>每100克碳水（g，可选）</span><input v-model="foodForm.carbohydrate" type="number" min="0" max="100" step="0.01" /></label><label><span>每100克蛋白质（g，可选）</span><input v-model="foodForm.protein" type="number" min="0" max="100" step="0.01" /></label><label><span>每100克脂肪（g，可选）</span><input v-model="foodForm.fat" type="number" min="0" max="100" step="0.01" /></label></div>
          <fieldset class="icon-picker-field">
            <legend>食品图标</legend>
            <div class="food-icon-mode"><button type="button" :class="{ selected: foodForm.icon_type === 'BUILTIN' }" @click="foodForm.icon_type = 'BUILTIN'">使用内置图标</button><button type="button" :class="{ selected: foodForm.icon_type === 'UPLOAD' }" @click="foodForm.icon_type = 'UPLOAD'">上传真实图片</button></div>
            <div v-if="foodForm.icon_type === 'BUILTIN'" class="icon-picker-grid"><button v-for="key in iconOrder" :key="key" type="button" class="icon-choice" :class="{ selected: foodForm.icon_value === key }" @click="foodForm.icon_value = key"><span class="material-pixel-icon" :style="iconStyle(key)" /><small>{{ iconLabels[key] }}</small></button></div>
            <label v-else class="food-upload-field">
              <img v-if="foodImagePreview" :src="foodImagePreview" alt="待上传食品图片" />
              <img v-else-if="editingFood?.icon_type === 'UPLOAD'" :src="`/uploads/${editingFood.icon_value}`" :alt="editingFood.name" />
              <span v-else>📷</span>
              <div><strong>{{ foodImage?.name ?? (editingFood?.icon_type === 'UPLOAD' ? '保留当前图片' : '选择食品图片') }}</strong><small>JPEG、PNG 或 WebP，最大 5MB</small></div>
              <input type="file" accept="image/jpeg,image/png,image/webp" @change="foodImage = ($event.target as HTMLInputElement).files?.[0] ?? null" />
            </label>
          </fieldset>
          <div class="modal-actions"><button type="button" class="secondary-button" @click="foodFormModal = false">取消</button><button class="primary-pixel-button compact" :disabled="savingFood">{{ savingFood ? "保存中…" : "保存食品" }}</button></div>
        </form>
      </section>
    </div>
  </div>
</template>
