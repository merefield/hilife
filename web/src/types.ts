export interface State {
 generation:number; population:number; births:number; deaths:number; stable:boolean; chunks:number; cache:number;
 name:string; rule:string; mode:string; theme:string; running:boolean; tps:number;
 camera:{x:number;y:number;zoom:number}; cells:[number,number,number,number][]; trails:[number,number,number][];
 showTrails:boolean;grid:boolean;wrap:boolean;width:number;height:number;undo:number;stepMs:number;revision:number;ant:{x:number;y:number};
}
export interface Config {presets:{name:string;description:string;rule:string;rle:string}[];modes:string[];themes:string[]}
export interface Action {type:string;value?:string;text?:string;number?:number;x?:number;y?:number;x2?:number;y2?:number;erase?:boolean}
