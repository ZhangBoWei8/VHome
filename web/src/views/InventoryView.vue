<script setup lang="ts">
import { PackagePlus, Plus, Search, Trash2, X } from "@lucide/vue";
import { computed, onMounted, ref, watch } from "vue";

import {
  createInventory,
  createLocation,
  createTemplate,
  discardInventory,
  listInventory,
  listLocations,
  listTemplates,
  updateInventory,
  type InventoryItem,
  type MaterialTemplate,
  type StorageLocation,
} from "@/api";
import pantryIcons from "@/assets/pantry-icons.png";

const iconOrder = [
  "milk",
  "egg",
  "leafy-vegetable",
  "potato",
  "scallion",
  "pork",
  "beef",
  "chicken",
  "fish",
  "fruit",
  "chips",
  "canned-food",
  "rice",
  "bread",
  "seasoning",
  "generic-food",
  "fridge",
  "pantry",
  "generic-storage",
  "drink",
];

const iconLabels: Record<string, string> = {
  milk: "牛奶",
  egg: "鸡蛋",
  "leafy-vegetable": "叶菜",
  potato: "土豆",
  scallion: "葱",
  pork: "猪肉",
  beef: "牛肉",
  chicken: "鸡肉",
  fish: "鱼类",
  fruit: "水果",
  chips: "零食",
  "canned-food": "罐头",
  rice: "米粮",
  bread: "面包",
  seasoning: "调味品",
  "generic-food": "通用食物",
  fridge: "冰箱",
  pantry: "仓库",
  "generic-storage": "通用位置",
  drink: "饮品",
};

const unitOptions = [
  { value: "G", label: "克（g）" },
  { value: "PIECE", label: "个" },
  { value: "PACK", label: "包" },
  { value: "BOTTLE", label: "瓶" },
];

function iconStyle(key: string) {
  const index = Math.max(0, iconOrder.indexOf(key));
  const column = index % 5;
  const row = Math.floor(index / 5);

  return {
    backgroundImage: `url(${pantryIcons})`,
    backgroundSize: "500% 400%",
    backgroundPosition: `${column * 25}% ${row * (100 / 3)}%`,
  };
}

function unitLabel(unit: string) {
  return unitOptions.find((option) => option.value === unit)?.label ?? unit;
}

const tabs = ["当前库存", "已过期", "常用物料"];
const activeTab = ref(tabs[0]);
const items = ref<InventoryItem[]>([]);
const templates = ref<MaterialTemplate[]>([]);
const locations = ref<StorageLocation[]>([]);
const search = ref("");
const order = ref("asc");
const modal = ref(false);
const templateModal = ref(false);
const editing = ref<InventoryItem | null>(null);
const error = ref("");
const templateError = ref("");

const today = () => new Date(Date.now() + 8 * 60 * 60 * 1000).toISOString().slice(0, 10);

const form = ref({
  template_id: "",
  location_id: "",
  name: "",
  icon_key: "generic-food",
  quantity: 1,
  unit: "G",
  stocked_on: today(),
  expires_on: "",
  calories: "",
  protein: "",
  fat: "",
  carbs: "",
  description: "",
  image: null as File | null,
});

const templateForm = ref({
  name: "",
  icon_key: "generic-food",
  default_unit: "G",
  cold: true,
  cold_days: 7,
  ambient: false,
  ambient_days: 7,
  calories: "",
  protein: "",
  fat: "",
  carbs: "",
});

const locationForm = ref({
  name: "",
  icon_key: "generic-storage",
  storage_type: "AMBIENT" as "COLD" | "AMBIENT",
});

const filtered = computed(() =>
  items.value.filter((item) =>
    item.name.toLowerCase().includes(search.value.trim().toLowerCase()),
  ),
);

const selectedTemplate = computed(() =>
  templates.value.find((item) => String(item.id) === form.value.template_id),
);

const supportedLocations = computed(() =>
  locations.value.filter((location) => {
    if (!selectedTemplate.value) return true;

    const shelfLife =
      location.storage_type === "COLD"
        ? selectedTemplate.value.cold_shelf_life_days
        : selectedTemplate.value.ambient_shelf_life_days;

    return shelfLife != null;
  }),
);

async function load() {
  error.value = "";

  try {
    [locations.value, templates.value, items.value] = await Promise.all([
      listLocations(),
      listTemplates(),
      listInventory(activeTab.value === "已过期" ? "expired" : "active", order.value),
    ]);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "读取物料失败";
  }
}

function addDays(date: string, days: number) {
  const value = new Date(`${date}T00:00:00Z`);
  value.setUTCDate(value.getUTCDate() + days);
  return value.toISOString().slice(0, 10);
}

function clearTemplateFields() {
  form.value.name = "";
  form.value.icon_key = "generic-food";
  form.value.unit = "G";
  form.value.expires_on = "";
  form.value.calories = "";
  form.value.protein = "";
  form.value.fat = "";
  form.value.carbs = "";
}

function applyTemplate() {
  const template = selectedTemplate.value;

  if (!template) {
    clearTemplateFields();
    return;
  }

  form.value.name = template.name;
  form.value.icon_key = template.icon_key;
  form.value.unit = template.default_unit;
  form.value.calories = template.calories_per_100g?.toString() ?? "";
  form.value.protein = template.protein_per_100g?.toString() ?? "";
  form.value.fat = template.fat_per_100g?.toString() ?? "";
  form.value.carbs = template.carbohydrate_per_100g?.toString() ?? "";

  const currentLocationIsSupported = supportedLocations.value.some(
    (location) => String(location.id) === form.value.location_id,
  );
  if (!currentLocationIsSupported) {
    form.value.location_id = String(supportedLocations.value[0]?.id ?? "");
  }

  applyExpiry();
}

function applyExpiry() {
  const template = selectedTemplate.value;
  const location = locations.value.find(
    (item) => String(item.id) === form.value.location_id,
  );
  if (!template || !location) return;

  const days =
    location.storage_type === "COLD"
      ? template.cold_shelf_life_days
      : template.ambient_shelf_life_days;

  form.value.expires_on = days
    ? addDays(form.value.stocked_on, days)
    : "";
}

watch(() => form.value.location_id, applyExpiry);
watch(() => form.value.stocked_on, applyExpiry);

function openAdd() {
  editing.value = null;
  form.value = {
    template_id: "",
    location_id: String(locations.value[0]?.id ?? ""),
    name: "",
    icon_key: "generic-food",
    quantity: 1,
    unit: "G",
    stocked_on: today(),
    expires_on: "",
    calories: "",
    protein: "",
    fat: "",
    carbs: "",
    description: "",
    image: null,
  };
  modal.value = true;
}

function openEdit(item: InventoryItem) {
  editing.value = item;
  form.value = {
    template_id: String(item.template_id ?? ""),
    location_id: String(item.storage_location_id),
    name: item.name,
    icon_key: item.icon_key,
    quantity: item.quantity,
    unit: item.unit,
    stocked_on: item.stocked_on.slice(0, 10),
    expires_on: item.expires_on.slice(0, 10),
    calories: item.calories_per_100g?.toString() ?? "",
    protein: item.protein_per_100g?.toString() ?? "",
    fat: item.fat_per_100g?.toString() ?? "",
    carbs: item.carbohydrate_per_100g?.toString() ?? "",
    description: item.description,
    image: null,
  };
  modal.value = true;
}

function toFormData() {
  const data = new FormData();

  Object.entries(form.value).forEach(([key, value]) => {
    if (value !== null && value !== "") {
      data.append(key, key === "image" ? (value as File) : String(value));
    }
  });

  if (editing.value) data.append("version", String(editing.value.version));
  return data;
}

async function save() {
  try {
    if (editing.value) {
      await updateInventory(editing.value.id, toFormData());
    } else {
      await createInventory(toFormData());
    }

    modal.value = false;
    await load();
    window.dispatchEvent(new Event("vhome:notifications-changed"));
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "保存失败";
  }
}

async function discard(item: InventoryItem) {
  if (!confirm(`确定丢弃“${item.name}”吗？`)) return;

  try {
    await discardInventory(item.id, item.version);
    await load();
    window.dispatchEvent(new Event("vhome:notifications-changed"));
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "丢弃失败";
  }
}

async function addTemplate() {
  templateError.value = "";
  if (!templateForm.value.cold && !templateForm.value.ambient) {
    templateError.value = "物料品类至少需要支持一种存储方式";
    return;
  }

  try {
    await createTemplate({
      name: templateForm.value.name,
      icon_key: templateForm.value.icon_key,
      default_unit: templateForm.value.default_unit,
      cold_days: templateForm.value.cold ? templateForm.value.cold_days : null,
      ambient_days: templateForm.value.ambient ? templateForm.value.ambient_days : null,
      calories: templateForm.value.calories === "" ? null : Number(templateForm.value.calories),
      protein: templateForm.value.protein === "" ? null : Number(templateForm.value.protein),
      fat: templateForm.value.fat === "" ? null : Number(templateForm.value.fat),
      carbs: templateForm.value.carbs === "" ? null : Number(templateForm.value.carbs),
    });
    templateForm.value.name = "";
    templateModal.value = false;
    await load();
  } catch (cause) {
    templateError.value = cause instanceof Error ? cause.message : "新增物料品类失败";
  }
}

function openTemplateModal() {
  templateError.value = "";
  templateForm.value = {
    name: "",
    icon_key: "generic-food",
    default_unit: "G",
    cold: true,
    cold_days: 7,
    ambient: false,
    ambient_days: 7,
    calories: "",
    protein: "",
    fat: "",
    carbs: "",
  };
  templateModal.value = true;
}

async function addLocation() {
  try {
    await createLocation(locationForm.value);
    locationForm.value.name = "";
    await load();
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "新增位置失败";
  }
}

watch([activeTab, order], () => {
  if (activeTab.value !== "常用物料") void load();
});
onMounted(load);
</script>

<template>
  <div class="inventory-page page-stack">
    <section class="inventory-hero">
      <div>
        <p class="eyebrow">FAMILY PANTRY</p>
        <h1>物料仓库</h1>
        <p>记录实际库存，让每一份食材在到期前被看见。</p>
      </div>
      <button class="primary-pixel-button compact" @click="openTemplateModal">
        <PackagePlus :size="18" />
        新增物料
      </button>
    </section>

    <p v-if="error" class="form-message error">{{ error }}</p>

    <section class="pixel-panel inventory-workspace">
      <div class="inventory-tabs">
        <button
          v-for="tab in tabs"
          :key="tab"
          :class="{ active: activeTab === tab }"
          @click="activeTab = tab"
        >
          {{ tab }}
        </button>
      </div>

      <template v-if="activeTab !== '常用物料'">
        <div class="inventory-toolbar">
          <div class="search-box">
            <Search />
            <input v-model="search" placeholder="搜索物料…" />
          </div>
          <select v-model="order" aria-label="库存排序方式">
            <option value="asc">即将到期优先</option>
            <option value="desc">保质期充足优先</option>
          </select>
        </div>

        <div class="inventory-items">
          <article
            v-for="item in filtered"
            :key="item.id"
            class="inventory-card real-card"
          >
            <img
              v-if="item.image_path"
              class="inventory-photo"
              :src="`/uploads/${item.image_path}`"
              :alt="`${item.name} 的照片`"
            />
            <span
              v-else
              class="material-pixel-icon"
              :style="iconStyle(item.icon_key)"
            />
            <span class="item-copy">
              <strong>{{ item.name }}</strong>
              <small>
                {{ item.location_name }} · {{ item.quantity }}
                {{ unitLabel(item.unit) }}
              </small>
              <small v-if="item.description" class="item-description">
                {{ item.description }}
              </small>
            </span>
            <span
              class="expiry-chip"
              :class="
                item.remaining_days <= 1
                  ? 'danger'
                  : item.remaining_days <= 3
                    ? 'warning'
                    : 'safe'
              "
            >
              {{
                item.remaining_days === 0
                  ? "今天到期"
                  : item.remaining_days < 0
                    ? "已过期"
                    : `剩 ${item.remaining_days} 天`
              }}
            </span>
            <div class="card-actions">
              <button @click="openEdit(item)">编辑</button>
              <button @click="discard(item)">
                <Trash2 :size="15" />
                丢弃
              </button>
            </div>
          </article>

          <button class="inventory-card add-card" @click="openAdd">
            <span><Plus /></span>
            <strong>添加库存</strong>
          </button>
        </div>
      </template>

      <template v-else>
        <div class="template-grid">
          <article
            v-for="template in templates"
            :key="template.id"
            class="template-card"
          >
            <span
              class="material-pixel-icon"
              :style="iconStyle(template.icon_key)"
            />
            <strong>{{ template.name }}</strong>
            <small>
              低温 {{ template.cold_shelf_life_days ?? "不支持" }} 天 · 常温
              {{ template.ambient_shelf_life_days ?? "不支持" }} 天
            </small>
          </article>
        </div>

        <div class="template-create-callout">
          <div>
            <strong>没有找到需要的物料？</strong>
            <small>新增后会成为永久品类，以后添加库存时可以直接复用。</small>
          </div>
          <button class="primary-pixel-button compact" @click="openTemplateModal">
            <Plus :size="17" />
            新增物料
          </button>
        </div>

        <form class="material-form compact-form" @submit.prevent="addLocation">
          <h3>新增存储位置</h3>
          <div class="form-grid">
            <label>
              <span>位置名称</span>
              <input v-model="locationForm.name" placeholder="例如：零食柜" required />
            </label>
            <label>
              <span>存储类型</span>
              <select v-model="locationForm.storage_type">
                <option value="COLD">低温</option>
                <option value="AMBIENT">常温</option>
              </select>
            </label>
          </div>

          <fieldset class="icon-picker-field compact-icon-picker">
            <legend>位置图标</legend>
            <div class="icon-picker-grid">
              <button
                v-for="key in iconOrder.slice(16, 19)"
                :key="key"
                type="button"
                class="icon-choice"
                :class="{ selected: locationForm.icon_key === key }"
                :aria-pressed="locationForm.icon_key === key"
                @click="locationForm.icon_key = key"
              >
                <span class="material-pixel-icon" :style="iconStyle(key)" />
                <small>{{ iconLabels[key] }}</small>
              </button>
            </div>
          </fieldset>

          <button class="secondary-button">新增位置</button>
        </form>
      </template>
    </section>

    <div
      v-if="templateModal"
      class="modal-backdrop"
      @click.self="templateModal = false"
    >
      <section class="modal-card material-modal">
        <header>
          <div>
            <h2>新增物料</h2>
            <small>创建可永久复用的物料品类</small>
          </div>
          <button type="button" aria-label="关闭新增物料窗口" data-tooltip-placement="left" @click="templateModal = false"><X /></button>
        </header>

        <form class="material-form" @submit.prevent="addTemplate">
          <p v-if="templateError" class="form-message error">
            {{ templateError }}
          </p>

          <div class="form-grid">
            <label>
              <span>物料名称</span>
              <input
                v-model="templateForm.name"
                placeholder="例如：燕麦奶"
                required
              />
            </label>
            <label>
              <span>默认单位</span>
              <select v-model="templateForm.default_unit">
                <option
                  v-for="option in unitOptions"
                  :key="option.value"
                  :value="option.value"
                >
                  {{ option.label }}
                </option>
              </select>
            </label>
          </div>

          <fieldset class="icon-picker-field">
            <legend>物料图标</legend>
            <div class="icon-picker-grid">
              <button
                v-for="key in iconOrder.slice(0, 16)"
                :key="key"
                type="button"
                class="icon-choice"
                :class="{ selected: templateForm.icon_key === key }"
                :aria-pressed="templateForm.icon_key === key"
                @click="templateForm.icon_key = key"
              >
                <span class="material-pixel-icon" :style="iconStyle(key)" />
                <small>{{ iconLabels[key] }}</small>
              </button>
            </div>
          </fieldset>

          <div class="form-grid storage-rule-grid">
            <label>
              <span>
                <input v-model="templateForm.cold" type="checkbox" />
                支持低温存储
              </span>
              <input
                v-model="templateForm.cold_days"
                type="number"
                min="1"
                :disabled="!templateForm.cold"
              />
            </label>
            <label>
              <span>
                <input v-model="templateForm.ambient" type="checkbox" />
                支持常温存储
              </span>
              <input
                v-model="templateForm.ambient_days"
                type="number"
                min="1"
                :disabled="!templateForm.ambient"
              />
            </label>
          </div>

          <div class="form-grid nutrition-grid">
            <label>
              <span>每100克热量（kcal）</span>
              <input
                v-model="templateForm.calories"
                type="number"
                min="0"
                step="0.01"
              />
            </label>
            <label>
              <span>每100克蛋白质（g）</span>
              <input
                v-model="templateForm.protein"
                type="number"
                min="0"
                step="0.01"
              />
            </label>
            <label>
              <span>每100克脂肪（g）</span>
              <input
                v-model="templateForm.fat"
                type="number"
                min="0"
                step="0.01"
              />
            </label>
            <label>
              <span>每100克碳水（g）</span>
              <input
                v-model="templateForm.carbs"
                type="number"
                min="0"
                step="0.01"
              />
            </label>
          </div>

          <p class="form-help">
            填写每100克热量后，该物料会同步加入食品库；
            如果食品库已有同名食品，则保留已有食品数据。
          </p>

          <div class="modal-actions">
            <button
              type="button"
              class="secondary-button"
              @click="templateModal = false"
            >
              取消
            </button>
            <button class="primary-pixel-button compact">保存物料</button>
          </div>
        </form>
      </section>
    </div>

    <div v-if="modal" class="modal-backdrop" @click.self="modal = false">
      <section class="modal-card material-modal">
        <header>
          <h2>{{ editing ? "编辑库存" : "添加库存" }}</h2>
          <button type="button" :aria-label="editing ? '关闭编辑库存窗口' : '关闭添加库存窗口'" data-tooltip-placement="left" @click="modal = false"><X /></button>
        </header>

        <form class="material-form" @submit.prevent="save">
          <label>
            <span>物料来源</span>
            <select
              v-model="form.template_id"
              :disabled="!!editing"
              @change="applyTemplate"
            >
              <option value="">新增物料（仅本次入库）</option>
              <option
                v-for="template in templates"
                :key="template.id"
                :value="String(template.id)"
              >
                常用品类 · {{ template.name }}
              </option>
            </select>
            <small v-if="editing">
              编辑时保留原品类关系，名称、期限和营养数据仍可单独调整
            </small>
            <small v-else>
              选择常用品类会自动填充；“新增物料”不会保存为永久品类。
            </small>
          </label>

          <div class="form-grid">
            <label>
              <span>物料名称</span>
              <input v-model="form.name" required />
            </label>
            <label>
              <span>存储位置</span>
              <select v-model="form.location_id" required>
                <option
                  v-for="location in supportedLocations"
                  :key="location.id"
                  :value="String(location.id)"
                >
                  {{ location.name }}（{{ location.storage_type === "COLD" ? "低温" : "常温" }}）
                </option>
              </select>
              <small
                v-if="selectedTemplate && supportedLocations.length === 0"
                class="danger-text"
              >
                该品类尚未配置可用的存储方式
              </small>
            </label>
          </div>

          <fieldset class="icon-picker-field modal-icon-picker">
            <legend>物料图标</legend>
            <div class="icon-picker-grid">
              <button
                v-for="key in iconOrder.slice(0, 16)"
                :key="key"
                type="button"
                class="icon-choice"
                :class="{ selected: form.icon_key === key }"
                :aria-pressed="form.icon_key === key"
                @click="form.icon_key = key"
              >
                <span class="material-pixel-icon" :style="iconStyle(key)" />
                <small>{{ iconLabels[key] }}</small>
              </button>
            </div>
          </fieldset>

          <div class="form-grid">
            <label>
              <span>数量</span>
              <input
                v-model="form.quantity"
                type="number"
                min="0.01"
                step="0.01"
                required
              />
            </label>
            <label>
              <span>单位</span>
              <select v-model="form.unit">
                <option
                  v-for="option in unitOptions"
                  :key="option.value"
                  :value="option.value"
                >
                  {{ option.label }}
                </option>
              </select>
            </label>
          </div>

          <div class="form-grid">
            <label>
              <span>入库日期</span>
              <input v-model="form.stocked_on" type="date" required />
            </label>
            <label>
              <span>到期日期</span>
              <input v-model="form.expires_on" type="date" required />
            </label>
          </div>

          <div class="form-grid nutrition-grid">
            <label>
              <span>每100克热量（kcal）</span>
              <input v-model="form.calories" type="number" min="0" step="0.01" />
            </label>
            <label>
              <span>每100克蛋白质（g）</span>
              <input v-model="form.protein" type="number" min="0" step="0.01" />
            </label>
            <label>
              <span>每100克脂肪（g）</span>
              <input v-model="form.fat" type="number" min="0" step="0.01" />
            </label>
            <label>
              <span>每100克碳水（g）</span>
              <input v-model="form.carbs" type="number" min="0" step="0.01" />
            </label>
          </div>

          <label>
            <span>描述（可选）</span>
            <textarea
              v-model="form.description"
              rows="3"
              placeholder="包装颜色、购买地点等…"
            />
          </label>
          <label>
            <span>照片（可选，最大 5MB）</span>
            <input
              type="file"
              accept="image/jpeg,image/png,image/webp"
              @change="
                form.image =
                  ($event.target as HTMLInputElement).files?.[0] ?? null
              "
            />
          </label>

          <div class="modal-actions">
            <button type="button" class="secondary-button" @click="modal = false">
              取消
            </button>
            <button class="primary-pixel-button compact">保存库存</button>
          </div>
        </form>
      </section>
    </div>
  </div>
</template>
