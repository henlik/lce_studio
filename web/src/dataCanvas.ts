export interface DataSelection {
  kind: 'record' | 'entity' | 'relationship';
  index: number;
}
export function dataEditorPlacement(anchor: {left:number; top:number; right:number; bottom:number}, width:number, height:number) {
  const panelWidth = Math.max(0,Math.min(390,width-16));
  const top = width < 650 ? Math.min(160,Math.max(8,height*.25)) : Math.max(8,Math.min(anchor.top,Math.max(8,height-300)));
  const availableHeight = Math.max(0,height-top-8);
  const left = anchor.right+12+panelWidth <= width-8 ? anchor.right+12
    : anchor.left-12-panelWidth >= 8 ? anchor.left-12-panelWidth
    : Math.max(8,width-panelWidth-8);
  return {left:width < 650 ? 8 : Math.max(8,Math.min(left,width-panelWidth-8)),top,width:panelWidth,maxHeight:availableHeight};
}
