// SPDX-License-Identifier: MIT

pragma solidity ^0.8.19;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract PartyBridge is ERC20, Ownable {
    uint256 private _cap;
    uint256 private _dailyMintCap;
    uint256 private _lastMintTimestamp;
    mapping(address => uint256) private _dailyMintedAmounts;

    event CapSet(uint256 cap);
    event DailyCapSet(uint256 dailyMintCap);

    //I never use the ownable contract so I'm not 100% sure on this one but maybe missing the Ownable() constructor call?
    //No input validation
    constructor(string memory name, string memory symbol, uint256 initialCap, uint256 initialDailyMintCap) ERC20(name, symbol) {
        require(initialCap > 0, "initialCap must be > 0");
        require(initialDailyMintCap > 0, "initialDailyMintCap must be > 0");
        _cap = initialCap;
        _dailyMintCap = initialDailyMintCap;
        _lastMintTimestamp = block.timestamp;
    }

    function cap() external view returns (uint256) {
        return _cap;
    }

    function dailyMintCap() external view returns (uint256) {
        return _dailyMintCap;
    }

    function lastMintTimestamp() external view returns (uint256) {
        return _lastMintTimestamp;
    }

    function dailyMintedAmount(address account) external view returns (uint256) {
        return _dailyMintedAmounts[account];
    }

    function mint(address account, uint256 amount) external onlyOwner {
        require(_dailyMintedAmounts[msg.sender] + amount <= _dailyMintCap, "TokenBase: daily mint cap exceeded");
        require(totalSupply() + amount <= _cap, "TokenBase: cap exceeded");
        _mint(account, amount);
        _dailyMintedAmounts[msg.sender] = _dailyMintedAmounts[msg.sender] + amount;
        _lastMintTimestamp = block.timestamp;
    }

    //I'd be worried about this one
    function burn(address account, uint256 amount) external onlyOwner { // Added burn method
        _burn(account, amount);
    }

    function setCap(uint256 newCap) external onlyOwner {
        require(newCap > 0, "Cap must be > 0");
        _cap = newCap;
        emit CapSet(newCap);
    }

    function setNewDailyMintCap(uint256 newCap) external onlyOwner {
        require(newCap > 0, "Cap must be > 0");
        _dailyMintCap = newCap;
        emit DailyCapSet(newCap);
    }

    fallback() external {
        revert("TokenBase: invalid function call");
    }
}
