export type CardTabIndicatorStyle = {
  transform: string;
  width: string;
  opacity: string;
};

const HIDDEN_STYLE: CardTabIndicatorStyle = {
  transform: 'translateX(0px)',
  width: '0px',
  opacity: '0',
};

export function hiddenCardTabIndicatorStyle(): CardTabIndicatorStyle {
  return { ...HIDDEN_STYLE };
}

export function calculateCardTabIndicatorStyle(
  container: HTMLElement | null
): CardTabIndicatorStyle {
  if (!container) {
    return hiddenCardTabIndicatorStyle();
  }

  const wrapper = container.querySelector('.n-tabs-wrapper') as HTMLElement | null;
  if (!wrapper) {
    return hiddenCardTabIndicatorStyle();
  }

  const activeTabElement = wrapper.querySelector(
    '.n-tabs-tab.n-tabs-tab--active'
  ) as HTMLElement | null;
  if (!activeTabElement) {
    return hiddenCardTabIndicatorStyle();
  }

  const wrapperRect = wrapper.getBoundingClientRect();
  const activeRect = activeTabElement.getBoundingClientRect();
  const tabWidth = activeRect.width;

  let indicatorWidth: number;
  if (tabWidth > 150) {
    indicatorWidth = tabWidth * 0.35;
  } else if (tabWidth > 100) {
    indicatorWidth = tabWidth * 0.45;
  } else if (tabWidth > 60) {
    indicatorWidth = tabWidth * 0.6;
  } else {
    indicatorWidth = tabWidth * 0.75;
  }
  indicatorWidth = Math.max(20, Math.min(80, indicatorWidth));

  const scrollContainer = container.querySelector('.v-x-scroll') as HTMLElement | null;
  const scrollLeft = scrollContainer?.scrollLeft ?? 0;
  const offsetLeft =
    activeRect.left - wrapperRect.left - scrollLeft + (tabWidth - indicatorWidth) / 2;

  return {
    transform: `translateX(${offsetLeft}px)`,
    width: `${indicatorWidth}px`,
    opacity: '1',
  };
}
