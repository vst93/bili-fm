// Cynhyrchwyd y ffeil hon yn awtomatig. PEIDIWCH Â MODIWL
// This file is automatically generated. DO NOT EDIT
import {main} from '../models';

export function CoinVideo(arg1:string,arg2:number):Promise<boolean>;

export function GetBLFavFolderList():Promise<Array<any>>;

export function GetBLFavFolderListDetail(arg1:number,arg2:number):Promise<Array<any>>;

export function GetBLFeedList(arg1:string):Promise<main.FeedList>;

export function GetBLRCMDList(arg1:number):Promise<main.RCMDList>;

export function GetBLUserInfo():Promise<main.UserInfo>;

export function GetBiliTicket(arg1:string):Promise<string>;

export function GetCList(arg1:string):Promise<main.VideoInfo>;

export function GetLoginQRCode():Promise<string>;

export function GetLoginQRCodeStatus():Promise<boolean>;

export function GetLoginStatus():Promise<boolean>;

export function GetSESSDATA():Promise<string>;

export function GetUpVideoList(arg1:number,arg2:string):Promise<main.FeedList>;

export function GetUrlByCid(arg1:number,arg2:number):Promise<main.PlayURLInfo>;

export function HasCoin(arg1:string):Promise<number>;

export function HasLiked(arg1:string):Promise<boolean>;

export function HmacSha256(arg1:string,arg2:string):Promise<string>;

export function LikeVideo(arg1:string,arg2:number):Promise<boolean>;

export function SearchVideo(arg1:string,arg2:string):Promise<Array<main.SearchResult>>;

export function SetLoginStatus(arg1:boolean):Promise<void>;

export function SetSESSDATA(arg1:string):Promise<void>;
