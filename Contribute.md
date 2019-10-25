### Contribution Guidelines

Please [comment in or create an issue](https://github.com/networkimprov/mnm-hammer/issues) 
and __wait for approval__ before creating a pull request.

Go code adheres to the [mnm codestyle](https://github.com/networkimprov/mnm/blob/master/codestyle.txt).

Vue template style:
- contained elements are indented with 3 spaces
   ```
   <el>
      <one>...
      <two>...
   </el>
   ```
- single-item elements chain closing tags
   ```
   <el>
      <one>...</one></el>
   <el>
      <one>
         ...
      </one></el>
   ```
- element attributes after the first get separate lines, aligned with the first attribute
   ```
   <el one="..."
       two="...">
   ```
- control-flow attributes appear first, then event handlers, then props, then html attributes
   ```
   <el v-if="..."
       @event="..."
       :prop="..."
       one="..."/>
   ```
- related, short attributes appear on the same line
   ```
   <el v-for="..." :key="..."
       class="..." :class="{...}">
   ```
- control-flow attributes always have a line-break
   ```
   <el v-if="...">
      ...</el>
   <el v-for="..." :key="...">
      ...</el>
   ```
- elements containing only text place the first `>` on the same line as the text
   ```
   <el>text</el>
   <el one="..."
       >text</el>
   <el one="..."
       two="...">text</el>
   ```
- `v-for` variables start with lowercase 'a'
   ```
   <el v-for="(a, aVal) in list">
      ...
   </el>
   ```

Vue component style:
- [tbd]
