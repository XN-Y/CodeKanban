import { ref, computed, onMounted, onUnmounted } from 'vue';

/**
 * 响应式断点定义 (与 Tailwind CSS 标准一致)
 */
export const BREAKPOINTS = {
  xs: 0, // < 640px  小屏手机
  sm: 640, // 640-767  大屏手机
  md: 768, // 768-1023 平板竖屏
  lg: 1024, // 1024-1199 平板横屏/小桌面
  xl: 1200, // >= 1200  桌面
} as const;

export type BreakpointKey = keyof typeof BREAKPOINTS;

/**
 * 响应式工具 composable
 * 提供设备类型检测和断点匹配功能
 */
export function useResponsive() {
  const windowWidth = ref(typeof window !== 'undefined' ? window.innerWidth : 1200);
  const windowHeight = ref(typeof window !== 'undefined' ? window.innerHeight : 800);

  // 当前断点
  const currentBreakpoint = computed<BreakpointKey>(() => {
    const width = windowWidth.value;
    if (width >= BREAKPOINTS.xl) return 'xl';
    if (width >= BREAKPOINTS.lg) return 'lg';
    if (width >= BREAKPOINTS.md) return 'md';
    if (width >= BREAKPOINTS.sm) return 'sm';
    return 'xs';
  });

  // 设备类型判断
  const isMobile = computed(() => windowWidth.value < BREAKPOINTS.md); // < 768px
  const isTablet = computed(
    () => windowWidth.value >= BREAKPOINTS.md && windowWidth.value < BREAKPOINTS.lg
  ); // 768-1023
  const isDesktop = computed(() => windowWidth.value >= BREAKPOINTS.lg); // >= 1024

  // 更细粒度的断点判断
  const isXs = computed(() => windowWidth.value < BREAKPOINTS.sm);
  const isSm = computed(
    () => windowWidth.value >= BREAKPOINTS.sm && windowWidth.value < BREAKPOINTS.md
  );
  const isMd = computed(
    () => windowWidth.value >= BREAKPOINTS.md && windowWidth.value < BREAKPOINTS.lg
  );
  const isLg = computed(
    () => windowWidth.value >= BREAKPOINTS.lg && windowWidth.value < BREAKPOINTS.xl
  );
  const isXl = computed(() => windowWidth.value >= BREAKPOINTS.xl);

  // 断点范围判断
  const isSmAndUp = computed(() => windowWidth.value >= BREAKPOINTS.sm);
  const isMdAndUp = computed(() => windowWidth.value >= BREAKPOINTS.md);
  const isLgAndUp = computed(() => windowWidth.value >= BREAKPOINTS.lg);
  const isXlAndUp = computed(() => windowWidth.value >= BREAKPOINTS.xl);

  const isSmAndDown = computed(() => windowWidth.value < BREAKPOINTS.md);
  const isMdAndDown = computed(() => windowWidth.value < BREAKPOINTS.lg);
  const isLgAndDown = computed(() => windowWidth.value < BREAKPOINTS.xl);

  // 触摸设备检测
  const isTouchDevice = computed(() => {
    if (typeof window === 'undefined') return false;
    return 'ontouchstart' in window || navigator.maxTouchPoints > 0;
  });

  // 横竖屏检测
  const isPortrait = computed(() => windowHeight.value > windowWidth.value);
  const isLandscape = computed(() => windowWidth.value >= windowHeight.value);

  // 更新窗口尺寸
  function updateWindowSize() {
    windowWidth.value = window.innerWidth;
    windowHeight.value = window.innerHeight;
  }

  // 自定义断点匹配
  function matchBreakpoint(minWidth?: number, maxWidth?: number): boolean {
    const width = windowWidth.value;
    if (minWidth !== undefined && width < minWidth) return false;
    if (maxWidth !== undefined && width >= maxWidth) return false;
    return true;
  }

  // 生命周期
  let resizeObserver: ResizeObserver | null = null;

  onMounted(() => {
    updateWindowSize();
    window.addEventListener('resize', updateWindowSize);

    // 使用 ResizeObserver 监听更精确的尺寸变化
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(updateWindowSize);
      resizeObserver.observe(document.body);
    }
  });

  onUnmounted(() => {
    window.removeEventListener('resize', updateWindowSize);
    resizeObserver?.disconnect();
  });

  return {
    // 尺寸
    windowWidth,
    windowHeight,

    // 当前断点
    currentBreakpoint,

    // 设备类型
    isMobile,
    isTablet,
    isDesktop,

    // 精确断点
    isXs,
    isSm,
    isMd,
    isLg,
    isXl,

    // 断点范围 (向上)
    isSmAndUp,
    isMdAndUp,
    isLgAndUp,
    isXlAndUp,

    // 断点范围 (向下)
    isSmAndDown,
    isMdAndDown,
    isLgAndDown,

    // 设备特性
    isTouchDevice,
    isPortrait,
    isLandscape,

    // 工具方法
    matchBreakpoint,
  };
}
