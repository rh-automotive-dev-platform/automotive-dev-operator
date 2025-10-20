# Charts Rules

Essential rules for PatternFly Charts implementation using Victory.js and ECharts.

## Installation Rules

### Required Installation
```bash
# ✅ Victory-based charts (recommended)
npm install @patternfly/react-charts victory

# ✅ ECharts-based charts (alternative)
npm install @patternfly/react-charts echarts

# ✅ Patternfly for charts css variables (recommended)
npm install @patternfly/patternfly
```

### Import Rules
- ✅ **CRITICAL: Use specific import paths** - Import from `/victory` or `/echarts` subdirectories
- ❌ **Don't use general imports** - Avoid importing from main package
- ⚠️ **Common Error**: AI assistants often forget the `/victory` path

```jsx
// ✅ Correct imports - MUST include /victory
import { ChartDonut, ChartLine, ChartBar } from '@patternfly/react-charts/victory';
import { EChart } from '@patternfly/react-charts/echarts';

// ❌ Wrong imports - Missing /victory will cause "Module not found" errors
import { ChartDonut } from '@patternfly/react-charts';
```

### Import PatternFly Charts CSS
   ```jsx
   // In your main App.js or index.js
   import '@patternfly/patternfly/patternfly-charts.css';
   ```

### Critical Import Path Troubleshooting

**⚠️ MOST COMMON ISSUE**: Chart components cannot be found

```bash
# Error message you'll see:
Module not found: Can't resolve '@patternfly/react-charts'
```

**Solutions in order:**
1. **Fix import paths** - Add `/victory` to your imports:
   ```jsx
   // Change this:
   import { ChartDonut } from '@patternfly/react-charts';
   
   // To this:
   import { ChartDonut } from '@patternfly/react-charts/victory';
   ```

2. **Install Victory dependency**:
   ```bash
   npm install victory
   ```

3. **Clear cache and reinstall**:
   ```bash
   rm -rf node_modules package-lock.json
   npm install
   ```

4. **Verify installation**:
   ```bash
   # Check if both packages are installed
   npm list @patternfly/react-charts
   npm list victory
   ```

## Chart Implementation Rules

### Color Rules
- ✅ **Use PatternFly chart color tokens** - For consistency with design system
- ❌ **Don't use hardcoded colors** - Use design tokens instead

```jsx
// ✅ Correct - Use PatternFly color tokens
const chartColors = [
  'var(--pf-t--chart--color--blue--300)',
  'var(--pf-t--chart--color--green--300)',
  'var(--pf-t--chart--color--orange--300)'
];

<ChartDonut data={data} colorScale={chartColors} />
```

### Responsive Rules
- ✅ **Implement responsive sizing** - Charts must work on all screen sizes
- ✅ **Use container-based dimensions** - Not fixed width/height
- ❌ **Don't hardcode dimensions** - Charts must be responsive

```jsx
// ✅ Required responsive pattern
const [dimensions, setDimensions] = useState({ width: 0, height: 0 });

useEffect(() => {
  const updateDimensions = () => {
    if (containerRef.current) {
      const { width, height } = containerRef.current.getBoundingClientRect();
      setDimensions({ width, height });
    }
  };
  updateDimensions();
  window.addEventListener('resize', updateDimensions);
  return () => window.removeEventListener('resize', updateDimensions);
}, []);
```

### Accessibility Rules
- ✅ **Provide ARIA labels** - For screen reader support
- ✅ **Use high contrast colors** - Meet WCAG standards
- ✅ **Support keyboard navigation** - Add tabIndex and role

```jsx
// ✅ Required accessibility pattern
<ChartDonut
  data={data}
  ariaDesc="Chart showing user distribution"
  ariaTitle="User Status Chart"
  tabIndex={0}
  role="img"
/>
```

### State Management Rules
- ✅ **Handle loading states** - Show spinners during data loading
- ✅ **Handle error states** - Show error messages with retry
- ✅ **Handle empty states** - Show appropriate empty messages
- ✅ **Use data memoization** - For performance optimization

```jsx
// ✅ Required state handling
if (isLoading) return <Spinner />;
if (error) return <EmptyState><EmptyStateHeader titleText="Chart error" /></EmptyState>;
if (!data?.length) return <EmptyState><EmptyStateHeader titleText="No data" /></EmptyState>;

const processedData = useMemo(() => {
  return rawData.map(item => ({ x: item.date, y: item.value }));
}, [rawData]);
```

### Integration Rules
- ✅ **Use with PatternFly components** - Integrate charts in Cards, PageSections
- ✅ **Follow grid layouts** - Use PatternFly grid for chart dashboards
- ❌ **Don't create standalone chart pages** - Integrate with PatternFly layout

```jsx
// ✅ Required integration pattern
import { Card, CardTitle, CardBody } from '@patternfly/react-core';

<Card>
  <CardTitle>Chart Title</CardTitle>
  <CardBody>
    <ChartDonut data={data} />
  </CardBody>
</Card>
```

## Performance Rules

### Required Optimizations
- ✅ **Use lazy loading for heavy charts** - Improve initial page load
- ✅ **Memoize data processing** - Use useMemo for expensive calculations
- ✅ **Implement proper loading states** - Show feedback during data loading

```jsx
// ✅ Required performance patterns
const LazyChart = lazy(() => import('./HeavyChart'));

<Suspense fallback={<Spinner />}>
  <LazyChart />
</Suspense>
```

## Essential Do's and Don'ts

### ✅ Do's
- Use PatternFly chart color tokens for consistency
- Implement responsive sizing for different screen sizes
- Provide proper ARIA labels and descriptions
- Handle loading, error, and empty states
- Use appropriate chart types for your data
- Optimize performance with data memoization
- Integrate charts with PatternFly layout components

### ❌ Don'ts
- Hardcode chart dimensions without responsive design
- Use colors that don't meet accessibility standards
- Skip loading states for charts with async data
- Ignore keyboard navigation and screen reader support
- Create overly complex charts
- Mix different charting libraries inconsistently
- Forget to handle empty data states

## Common Issues

### Module Not Found
- **Clear cache**: `rm -rf node_modules package-lock.json`
- **Reinstall**: `npm install`
- **Check paths**: Verify import paths are correct

### Chart Not Rendering
- **Check container dimensions**: Ensure parent has width/height
- **Verify data format**: Data must match chart expectations
- **Check console**: Look for Victory.js or ECharts warnings

### Performance Issues
- **Use data memoization**: useMemo for expensive calculations
- **Implement lazy loading**: For heavy chart components
- **Optimize re-renders**: Use React.memo for chart components

#### Issue: Chart colors not correct
```bash
# Symptoms: Chart color variables (--pf-v6-chart-color-blue-100, --pf-t--chart--color--blue--300) are not defined
```

**Solutions**:
1. **Install patternfly package**:
   ```bash
   npm install @patternfly/patternfly
   ```

2. **Import PatternFly Charts CSS**:
   ```jsx
   // In your main App.js or index.js
   import '@patternfly/patternfly/patternfly-charts.css';
   ```
  
## Quick Reference
- **[PatternFly Charts README](https://github.com/patternfly/patternfly-react/tree/main/packages/react-charts#readme)** - Installation and usage
- **[Victory.js Documentation](https://formidable.com/open-source/victory/)** - Chart library documentation
- **[PatternFly Chart Guidelines](https://www.patternfly.org/charts/about)** - Design guidelines

## Reference Documentation

- [PatternFly Charts on PatternFly.org](https://www.patternfly.org/charts/about)
- [PatternFly React Charts GitHub Repository](https://github.com/patternfly/patternfly-react/tree/main/packages/react-charts)

> For the most up-to-date documentation and code examples, consult both PatternFly.org and the official GitHub repository. When using AI tools, leverage context7 to fetch the latest docs from these sources.