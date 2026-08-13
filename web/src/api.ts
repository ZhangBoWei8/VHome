export type MemberRole = "OWNER" | "ADMIN" | "MEMBER";

export type MemberStatus =
  | "PENDING"
  | "ACTIVE"
  | "REJECTED"
  | "DISABLED";

export interface BootstrapData {
  initialized: boolean;
  authenticated: boolean;
  next_route: "SETUP" | "LOGIN" | "DASHBOARD";
  api_version: string;
}

export interface IconData {
  type: string;
  value: string;
}

export interface HouseholdData {
  id: number;
  name: string;
  icon: IconData;
  province: string;
  city: string;
  version: number;
}

export interface MemberData {
  id: number;
  username: string;
  display_name: string;
  role: MemberRole;
  status: MemberStatus;
  presence_status: PresenceStatus | "";
  avatar_key: MemberAvatar;
  version: number;
}

export type MemberAvatar =
  | "initials"
  | "man"
  | "woman"
  | "boy"
  | "girl"
  | "dog";

export type PresenceStatus =
  | "HOME"
  | "SCHOOL"
  | "WORKING"
  | "OUT"
  | "NAPPING"
  | "RESTING"
  | "SICK"
  | "STUDYING";

export interface SessionData {
  csrf_token?: string;
  expires_at: string;
  household: HouseholdData;
  member: MemberData;
}

export interface RegistrationData {
  member: MemberData;
}

interface APIErrorData {
  code: string;
  message: string;
}

interface APIEnvelope<T> {
  data?: T;
  error?: APIErrorData;
}

export class APIError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
  }
}

const apiBaseURL = "/api/v1";
const csrfCookieName = "vhome_session_csrf";

function readCookie(name: string): string {
  const prefix = `${encodeURIComponent(name)}=`;
  const item = document.cookie
    .split("; ")
    .find((value) => value.startsWith(prefix));

  if (!item) return "";

  return decodeURIComponent(item.slice(prefix.length));
}

function isUnsafeMethod(method: string): boolean {
  return !["GET", "HEAD", "OPTIONS"].includes(method.toUpperCase());
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const method = options.method ?? "GET";
  const headers = new Headers(options.headers);

  if (options.body !== undefined && !(options.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }

  if (isUnsafeMethod(method)) {
    const csrfToken = readCookie(csrfCookieName);
    if (csrfToken) {
      headers.set("X-CSRF-Token", csrfToken);
    }
  }

  const response = await fetch(`${apiBaseURL}${path}`, {
    ...options,
    method,
    headers,
    credentials: "include",
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const payload = (await response.json()) as APIEnvelope<T>;

  if (!response.ok) {
    throw new APIError(
      response.status,
      payload.error?.code ?? "UNKNOWN_ERROR",
      payload.error?.message ?? "请求失败",
    );
  }

  if (payload.data === undefined) {
    throw new APIError(response.status, "INVALID_RESPONSE", "服务端响应缺少 data");
  }

  return payload.data;
}

export function getBootstrap(): Promise<BootstrapData> {
  return request<BootstrapData>("/bootstrap");
}

export function setup(input: {
  householdName: string;
  householdPassword: string;
  householdAvatar?: string;
  ownerName: string;
  ownerPassword: string;
}): Promise<SessionData> {
  return request<SessionData>("/setup", {
    method: "POST",
    body: JSON.stringify({
      household: {
        name: input.householdName,
        password: input.householdPassword,
        avatar: input.householdAvatar ?? "house",
      },
      owner: {
        name: input.ownerName,
        password: input.ownerPassword,
      },
    }),
  });
}

export function login(input: { name: string; password: string }): Promise<SessionData> {
  return request<SessionData>("/auth/sessions", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function registerMember(input: {
  name: string;
  password: string;
  householdPassword: string;
}): Promise<RegistrationData> {
  return request<RegistrationData>("/members/registrations", {
    method: "POST",
    body: JSON.stringify({
      name: input.name,
      password: input.password,
      household_password: input.householdPassword,
    }),
  });
}

export function getCurrentSession(): Promise<SessionData> {
  return request<SessionData>("/auth/session");
}

export function logout(): Promise<void> {
  return request<void>("/auth/session", { method: "DELETE" });
}

export interface StorageLocation {
  id: number; name: string; icon_key: string; storage_type: "COLD" | "AMBIENT";
  is_builtin: boolean; enabled: boolean;
}
export interface MaterialTemplate {
  id: number; name: string; icon_key: string; default_unit: string;
  cold_shelf_life_days: number | null; ambient_shelf_life_days: number | null;
  calories_per_100g: number | null; protein_per_100g: number | null;
  fat_per_100g: number | null; carbohydrate_per_100g: number | null;
  source: string; enabled: boolean; version: number;
}
export interface InventoryItem {
  id: number; template_id: number | null; storage_location_id: number;
  name: string; icon_key: string; quantity: number; unit: string;
  stocked_on: string; expires_on: string; calories_per_100g: number | null;
  protein_per_100g: number | null; fat_per_100g: number | null;
  carbohydrate_per_100g: number | null; description: string; image_path: string;
  status: string; version: number; location_name: string; storage_type: "COLD" | "AMBIENT";
  remaining_days: number; total_days: number;
}
export interface MaterialReminder { item_id: number; item_name: string; milestone: string; remaining_days: number; message: string }
export interface NotificationSummary { pending_members: number; material_reminders: MaterialReminder[] }

export const listMembers = () => request<MemberData[]>("/members");
export const approveMember = (id: number, version: number) => request<MemberData>(`/members/${id}/approve`, { method: "POST", body: JSON.stringify({ version }) });
export const rejectMember = (id: number, version: number) => request<MemberData>(`/members/${id}/reject`, { method: "POST", body: JSON.stringify({ version }) });
export const changeMemberRole = (id: number, version: number, role: MemberRole) => request<MemberData>(`/members/${id}/role`, { method: "PATCH", body: JSON.stringify({ version, role }) });
export const disableMember = (id: number, version: number) => request<MemberData>(`/members/${id}`, { method: "DELETE", body: JSON.stringify({ version }) });

export const listLocations = () => request<StorageLocation[]>("/storage-locations");
export const createLocation = (input: {name:string;icon_key:string;storage_type:"COLD"|"AMBIENT"}) => request<StorageLocation>("/storage-locations", {method:"POST",body:JSON.stringify(input)});
export const listTemplates = () => request<MaterialTemplate[]>("/material-templates");
export const createTemplate = (input: Record<string, unknown>) => request<MaterialTemplate>("/material-templates", {method:"POST",body:JSON.stringify(input)});
export const listInventory = (scope="active", order="asc") => request<InventoryItem[]>(`/inventory-items?scope=${scope}&order=${order}`);
export const createInventory = (form: FormData) => request<InventoryItem>("/inventory-items", {method:"POST",body:form});
export const updateInventory = (id:number, form:FormData) => request<InventoryItem>(`/inventory-items/${id}`, {method:"PATCH",body:form});
export const discardInventory = (id:number,version:number,reason="") => request<void>(`/inventory-items/${id}/discard`, {method:"POST",body:JSON.stringify({version,reason})});
export const getNotifications = () => request<NotificationSummary>("/notifications");
export const readMaterialReminder = (itemID:number,milestone:string) => request<void>(`/notifications/materials/${itemID}/${milestone}/read`, {method:"POST",body:JSON.stringify({})});

export interface DashboardStorage {
  storage_type: "COLD" | "AMBIENT";
  location_count: number;
  item_count: number;
  percentage: number;
}
export interface DashboardActivity {
  type: "INVENTORY_ADDED" | "INVENTORY_DISCARDED" | "MEMBER_JOINED";
  text: string;
  happened_at: string;
}
export interface DashboardMember {
  id: number;
  display_name: string;
  role: MemberRole;
  presence_status: PresenceStatus | "";
  avatar_key: MemberAvatar;
  version: number;
}
export interface WeatherSummary {
  available: boolean;
  city: string;
  temperature_mean: number;
  weather_code: number;
  description: string;
  icon: string;
  date: string;
}
export interface DashboardSummary {
  household_name: string;
  province: string;
  city: string;
  inventory_count: number;
  attention_count: number;
  due_today_count: number;
  pantry_watch: InventoryItem[];
  storage: DashboardStorage[];
  activities: DashboardActivity[];
  members: DashboardMember[];
  weather: WeatherSummary;
}
export interface HouseholdSettings {
  id: number;
  login_name: string;
  name: string;
  province: string;
  city: string;
  version: number;
}

export const getDashboard = () => request<DashboardSummary>("/dashboard");
export const getHouseholdSettings = () => request<HouseholdSettings>("/household/settings");
export const updateHouseholdSettings = (input: {name:string;province:string;city:string;version:number}) =>
  request<HouseholdSettings>("/household/settings", {method:"PATCH",body:JSON.stringify(input)});
export const getProfile = () => request<MemberData>("/members/me/profile");
export const updateProfile = (input: {
  display_name: string;
  avatar_key: MemberAvatar;
  presence_status: PresenceStatus | "";
  version: number;
}) => request<MemberData>("/members/me/profile", {method:"PATCH",body:JSON.stringify(input)});

export type FoodIconType = "BUILTIN" | "UPLOAD";
export type FoodSource = "BUILTIN" | "USER";
export type MealType = "BREAKFAST" | "LUNCH" | "DINNER" | "SNACK";

export interface Food {
  id: number;
  name: string;
  calories_per_100g: number;
  carbohydrate_per_100g: number | null;
  protein_per_100g: number | null;
  fat_per_100g: number | null;
  icon_type: FoodIconType;
  icon_value: string;
  source: FoodSource;
  created_by: number | null;
  deleted_by: number | null;
  version: number;
  deleted_at: string | null;
  created_at: string;
  updated_at: string;
  nutrition_complete: boolean;
  estimated_calories_per_100g: number | null;
  nutrition_mismatch: boolean;
}

export interface MealMemberOption {
  id: number;
  display_name: string;
  avatar_key: MemberAvatar;
  is_current: boolean;
}

export interface MealRecord {
  id: number;
  member_id: number;
  meal_date: string;
  meal_type: MealType;
  food_id: number | null;
  food_name_snapshot: string;
  icon_type_snapshot: FoodIconType;
  icon_value_snapshot: string;
  weight_grams: number;
  calories_per_100g_snapshot: number;
  carbohydrate_per_100g_snapshot: number | null;
  protein_per_100g_snapshot: number | null;
  fat_per_100g_snapshot: number | null;
  created_by: number;
  deleted_by: number | null;
  version: number;
  deleted_at: string | null;
  created_at: string;
  updated_at: string;
  calories: number;
  carbohydrate: number | null;
  protein: number | null;
  fat: number | null;
  nutrition_incomplete: boolean;
}

export interface DailyMealSummary {
  calories: number;
  carbohydrate: number;
  protein: number;
  fat: number;
  nutrition_incomplete: boolean;
  incomplete_record_count: number;
  record_count: number;
}

export interface MealDay {
  date: string;
  member: MealMemberOption;
  summary: DailyMealSummary;
  records: MealRecord[];
}

export interface MealCalendarDay {
  date: string;
  calories: number;
  record_count: number;
  nutrition_incomplete: boolean;
  incomplete_record_count: number;
}

export interface MealCalendar {
  member_id: number;
  month: string;
  days: MealCalendarDay[];
}

export interface MealRecordInput {
  meal_date: string;
  meal_type: MealType;
  food_id: number;
  weight_grams: number;
  version?: number;
}

export const listFoods = (scope: "ACTIVE" | "DELETED" = "ACTIVE", keyword = "") => {
  const query = new URLSearchParams({ scope, keyword });
  return request<Food[]>(`/foods?${query.toString()}`);
};
export const getFood = (id: number) => request<Food>(`/foods/${id}`);
export const createFood = (form: FormData) => request<Food>("/foods", { method: "POST", body: form });
export const updateFood = (id: number, form: FormData) => request<Food>(`/foods/${id}`, { method: "PATCH", body: form });
export const deleteFood = (id: number, version: number) => request<Food>(`/foods/${id}`, { method: "DELETE", body: JSON.stringify({ version }) });
export const restoreFood = (id: number, version: number) => request<Food>(`/foods/${id}/restore`, { method: "POST", body: JSON.stringify({ version }) });

export const listMealMemberOptions = () => request<MealMemberOption[]>("/meals/member-options");
export const getMyMealDay = (date: string) => request<MealDay>(`/meals/me?date=${encodeURIComponent(date)}`);
export const getMyMealCalendar = (month: string) => request<MealCalendar>(`/meals/me/calendar?month=${encodeURIComponent(month)}`);
export const getMemberMealToday = (memberID: number) => request<MealDay>(`/meals/members/${memberID}/today`);
export const createMyMealRecord = (input: MealRecordInput) => request<MealRecord>("/meals/me/records", { method: "POST", body: JSON.stringify(input) });
export const updateMyMealRecord = (id: number, input: MealRecordInput) => request<MealRecord>(`/meals/me/records/${id}`, { method: "PATCH", body: JSON.stringify(input) });
export const deleteMyMealRecord = (id: number, version: number) => request<void>(`/meals/me/records/${id}`, { method: "DELETE", body: JSON.stringify({ version }) });
