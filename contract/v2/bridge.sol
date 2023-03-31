// SPDX-License-Identifier: MIT
pragma solidity ^0.8.7;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/math/SafeMath.sol";

contract TokenBase is ERC20, Ownable {
    using SafeMath for uint256;

    uint256 private _cap;
    uint256 private _dailyMintCap;
    uint256 private _lastMintTimestamp;
    mapping(address => uint256) private _dailyMintedAmounts;

    event CapSet(uint256 cap);

    constructor(string memory name, string memory symbol, uint256 cap, uint256 dailyMintCap) ERC20(name, symbol) {
        _cap = cap;
        _dailyMintCap = dailyMintCap;
        _lastMintTimestamp = block.timestamp;
    }

    function cap() public view returns (uint256) {
        return _cap;
    }

    function dailyMintCap() public view returns (uint256) {
        return _dailyMintCap;
    }

    function lastMintTimestamp() public view returns (uint256) {
        return _lastMintTimestamp;
    }

    function dailyMintedAmount(address account) public view returns (uint256) {
        return _dailyMintedAmounts[account];
    }

    function mint(address account, uint256 amount) external onlyOwner {
        require(block.timestamp >= _lastMintTimestamp.add(1 days), "TokenBase: minting not allowed yet today");
        require(_dailyMintedAmounts[msg.sender].add(amount) <= _dailyMintCap, "TokenBase: daily mint cap exceeded");
        require(totalSupply().add(amount) <= _cap, "TokenBase: cap exceeded");
        _mint(account, amount);
        _dailyMintedAmounts[msg.sender] = _dailyMintedAmounts[msg.sender].add(amount);
        _lastMintTimestamp = block.timestamp;
    }

    function setCap(uint256 cap) external onlyOwner {
        _cap = cap;
        emit CapSet(cap);
    }

    fallback() external {
        revert("TokenBase: invalid function call");
    }
}